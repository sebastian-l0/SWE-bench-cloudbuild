package orchestrator

import (
	"context"
	"sync"

	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/manifest"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/model"
)

// phaseForLayer maps a layer to its run phase.
func phaseForLayer(layer string) string {
	switch layer {
	case manifest.LayerBase:
		return PhaseBuildingBase
	case manifest.LayerEnv:
		return PhaseBuildingEnv
	default:
		return PhaseBuildingInstance
	}
}

// CreateImageBuilds persists ImageBuild rows for a run from a parsed manifest and
// the prepared per-layer pipeline IDs.
func (o *Orchestrator) CreateImageBuilds(ctx context.Context, run model.Run, m *manifest.Manifest, lp layerPipelines) error {
	for _, img := range m.Images {
		now := o.now()
		rec := model.ImageBuild{
			ID:           o.newID(),
			RunID:        run.ID,
			Layer:        img.Layer,
			LocalKey:     img.LocalKey,
			TargetImage:  img.TargetImage,
			ContextPath:  img.ContextPath,
			Dockerfile:   img.Dockerfile,
			DependsOnKey: img.DependsOnKey,
			WorkspaceID:  lp.workspaceID,
			PipelineID:   lp.byLayer[img.Layer],
			Status:       StatusPending,
			RawManifest:  img.RawJSON,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := o.store.CreateImageBuild(ctx, rec); err != nil {
			return err
		}
	}
	return nil
}

// runLayer builds all images in a layer using a bounded worker pool. It returns
// true when every image succeeded.
func (o *Orchestrator) runLayer(ctx context.Context, layer string, images []model.ImageBuild) bool {
	limit := o.concurrency.forLayer(layer)
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	var mu sync.Mutex
	allOK := true

	for i := range images {
		img := images[i]
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			o.markImage(ctx, &img, StatusQueued, "")
			status, err := o.buildImage(ctx, &img)
			msg := ""
			if err != nil {
				msg = err.Error()
			}
			o.markImage(ctx, &img, status, msg)

			if status != StatusSuccess {
				mu.Lock()
				allOK = false
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return allOK
}

// markImage updates an image build's status and error and persists it.
func (o *Orchestrator) markImage(ctx context.Context, img *model.ImageBuild, status, errMsg string) {
	img.Status = status
	img.Error = errMsg
	img.UpdatedAt = o.now()
	_ = o.store.UpdateImageBuild(ctx, *img)
}

// skipLayer marks every image in a layer as skipped.
func (o *Orchestrator) skipLayer(ctx context.Context, images []model.ImageBuild) {
	for i := range images {
		img := images[i]
		o.markImage(ctx, &img, StatusSkipped, "upstream layer failed")
	}
}

// Build runs the full CP orchestration for a run: prepare resources, persist
// image builds, then build base -> env -> instance under a strict gate. The run
// record is updated as phases progress and on terminal state.
func (o *Orchestrator) Build(ctx context.Context, run model.Run, m *manifest.Manifest) error {
	o.setRunPhase(ctx, &run, PhasePreparingCP, StatusRunning)

	lp, err := o.PrepareResources(ctx, run, m.Images)
	if err != nil {
		o.failRun(ctx, &run, err)
		return err
	}
	if err := o.CreateImageBuilds(ctx, run, m, lp); err != nil {
		o.failRun(ctx, &run, err)
		return err
	}

	stored, err := o.store.ListImageBuildsByRun(ctx, run.ID)
	if err != nil {
		o.failRun(ctx, &run, err)
		return err
	}
	groups := sortImagesByLayer(stored)

	layers := []string{manifest.LayerBase, manifest.LayerEnv, manifest.LayerInstance}
	gateFailed := false
	for i, layer := range layers {
		imgs := groups[layer]
		if len(imgs) == 0 {
			continue
		}
		if gateFailed {
			o.skipLayer(ctx, imgs)
			continue
		}
		o.setRunPhase(ctx, &run, phaseForLayer(layer), StatusRunning)
		if ok := o.runLayer(ctx, layer, imgs); !ok {
			gateFailed = true
			// Mark all downstream layers skipped.
			for _, downstream := range layers[i+1:] {
				o.skipLayer(ctx, groups[downstream])
			}
		}
	}

	if gateFailed {
		o.failRun(ctx, &run, errLayerFailed)
		return errLayerFailed
	}
	o.completeRun(ctx, &run)
	return nil
}

func (o *Orchestrator) setRunPhase(ctx context.Context, run *model.Run, phase, status string) {
	run.Phase = phase
	run.Status = status
	if run.StartedAt == nil {
		now := o.now()
		run.StartedAt = &now
	}
	_ = o.store.UpdateRun(ctx, *run)
}

func (o *Orchestrator) failRun(ctx context.Context, run *model.Run, err error) {
	now := o.now()
	run.Status = StatusFailed
	run.Error = err.Error()
	run.FinishedAt = &now
	_ = o.store.UpdateRun(ctx, *run)
}

func (o *Orchestrator) completeRun(ctx context.Context, run *model.Run) {
	now := o.now()
	run.Status = StatusSuccess
	run.FinishedAt = &now
	_ = o.store.UpdateRun(ctx, *run)
}

// RetryImage re-runs a failed image build, creating a new attempt.
func (o *Orchestrator) RetryImage(ctx context.Context, imageBuildID string) error {
	img, err := o.store.GetImageBuild(ctx, imageBuildID)
	if err != nil {
		return err
	}
	o.markImage(ctx, &img, StatusQueued, "")
	status, buildErr := o.buildImage(ctx, &img)
	msg := ""
	if buildErr != nil {
		msg = buildErr.Error()
	}
	o.markImage(ctx, &img, status, msg)
	return buildErr
}

// CancelRun cancels running pipeline runs best-effort and marks the run canceled.
func (o *Orchestrator) CancelRun(ctx context.Context, runID string) error {
	images, err := o.store.ListImageBuildsByRun(ctx, runID)
	if err != nil {
		return err
	}
	for i := range images {
		img := images[i]
		if img.Status == StatusRunning || img.Status == StatusQueued {
			if img.LastRunID != "" {
				_ = o.cp.CancelPipelineRun(ctx, img.WorkspaceID, img.PipelineID, img.LastRunID)
			}
			o.markImage(ctx, &img, StatusCanceled, "canceled by user")
		}
	}
	run, err := o.store.GetRun(ctx, runID)
	if err != nil {
		return err
	}
	now := o.now()
	run.Status = StatusCanceled
	run.FinishedAt = &now
	return o.store.UpdateRun(ctx, run)
}

// ImageLog fetches redacted log content for an image's latest run.
func (o *Orchestrator) ImageLog(ctx context.Context, imageBuildID string) (string, error) {
	img, err := o.store.GetImageBuild(ctx, imageBuildID)
	if err != nil {
		return "", err
	}
	if img.LastRunID == "" {
		return "", nil
	}
	run, err := o.cp.GetPipelineRun(ctx, img.WorkspaceID, img.PipelineID, img.LastRunID)
	if err != nil {
		return "", err
	}
	var buf string
	for _, stage := range run.Stages {
		for _, task := range stage.Tasks {
			page, err := o.cp.GetTaskRunLog(ctx, img.WorkspaceID, task.ID, "")
			if err != nil {
				return "", err
			}
			buf += page.Content
		}
	}
	return buf, nil
}
