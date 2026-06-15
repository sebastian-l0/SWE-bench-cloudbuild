package orchestrator

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/manifest"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/model"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/store"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/volc/cp"
)

// Run phases.
const (
	PhasePreparingCP      = "preparing_cp_resources"
	PhaseBuildingBase     = "building_base_images"
	PhaseBuildingEnv      = "building_env_images"
	PhaseBuildingInstance = "building_instance_images"
)

// Run / image statuses.
const (
	StatusPending  = "pending"
	StatusQueued   = "queued"
	StatusRunning  = "running"
	StatusSuccess  = "success"
	StatusFailed   = "failed"
	StatusSkipped  = "skipped"
	StatusCanceled = "canceled"
)

// Concurrency configures per-layer worker pools.
type Concurrency struct {
	Base     int
	Env      int
	Instance int
}

func (c Concurrency) forLayer(layer string) int {
	switch layer {
	case manifest.LayerBase:
		return max1(c.Base)
	case manifest.LayerEnv:
		return max1(c.Env)
	default:
		return max1(c.Instance)
	}
}

func max1(n int) int {
	if n < 1 {
		return 1
	}
	return n
}

// IDGen generates unique IDs. Tests can inject a deterministic generator.
type IDGen func() string

// Orchestrator builds a run's images via CP under a strict layer gate.
type Orchestrator struct {
	cp          cp.Client
	store       store.Store
	settings    BuildSettings
	concurrency Concurrency
	newID       IDGen
	now         func() time.Time
	// pollInterval between CP status polls.
	pollInterval time.Duration
	// pollTimeout caps how long a single image build is polled.
	pollTimeout time.Duration
}

// Options configures an Orchestrator.
type Options struct {
	Client       cp.Client
	Store        store.Store
	Settings     BuildSettings
	Concurrency  Concurrency
	IDGen        IDGen
	Now          func() time.Time
	PollInterval time.Duration
	PollTimeout  time.Duration
}

// New builds an Orchestrator with sane defaults.
func New(opts Options) *Orchestrator {
	o := &Orchestrator{
		cp:           opts.Client,
		store:        opts.Store,
		settings:     opts.Settings,
		concurrency:  opts.Concurrency,
		newID:        opts.IDGen,
		now:          opts.Now,
		pollInterval: opts.PollInterval,
		pollTimeout:  opts.PollTimeout,
	}
	if o.newID == nil {
		var mu sync.Mutex
		seq := 0
		o.newID = func() string {
			mu.Lock()
			defer mu.Unlock()
			seq++
			return fmt.Sprintf("id-%d", seq)
		}
	}
	if o.now == nil {
		o.now = time.Now
	}
	if o.pollInterval == 0 {
		o.pollInterval = 2 * time.Second
	}
	if o.pollTimeout == 0 {
		o.pollTimeout = 2 * time.Hour
	}
	return o
}

// layerPipelines holds the per-layer pipeline IDs created during preparation.
type layerPipelines struct {
	workspaceID string
	byLayer     map[string]string
}

// PrepareResources creates a fresh workspace and one pipeline per layer, then
// records workspace/pipeline IDs onto the run's image builds.
func (o *Orchestrator) PrepareResources(ctx context.Context, run model.Run, images []manifest.Image) (layerPipelines, error) {
	lp := layerPipelines{byLayer: map[string]string{}}

	ws, err := o.cp.CreateWorkspace(ctx, cp.CreateWorkspaceInput{Name: "swe-" + run.ID})
	if err != nil {
		return lp, fmt.Errorf("orchestrator: create workspace: %w", err)
	}
	lp.workspaceID = ws.ID

	for _, layer := range []string{manifest.LayerBase, manifest.LayerEnv, manifest.LayerInstance} {
		if !layerPresent(images, layer) {
			continue
		}
		p, err := o.cp.CreatePipeline(ctx, createPipelineInput(ws.ID, fmt.Sprintf("swe-%s-%s", layer, run.ID), layer, o.settings))
		if err != nil {
			return lp, fmt.Errorf("orchestrator: create %s pipeline: %w", layer, err)
		}
		lp.byLayer[layer] = p.ID
	}
	return lp, nil
}

func layerPresent(images []manifest.Image, layer string) bool {
	for _, img := range images {
		if img.Layer == layer {
			return true
		}
	}
	return false
}

// buildImage triggers a pipeline run for one image and polls until terminal.
// It returns the final status (success/failed/canceled).
func (o *Orchestrator) buildImage(ctx context.Context, img *model.ImageBuild) (string, error) {
	attemptID := o.newID()
	now := o.now()
	attempt := model.RunAttempt{
		ID:           attemptID,
		ImageBuildID: img.ID,
		Status:       StatusRunning,
		StartedAt:    now,
		UpdatedAt:    now,
	}
	if err := o.store.CreateAttempt(ctx, attempt); err != nil {
		return StatusFailed, err
	}

	pr, err := o.cp.RunPipeline(ctx, cp.RunPipelineInput{
		PipelineID: img.PipelineID,
		Params:     runParams(img.Layer, img.LocalKey, o.settings),
	})
	if err != nil {
		o.finishAttempt(ctx, &attempt, StatusFailed)
		return StatusFailed, err
	}

	img.LastRunID = pr.ID
	img.Attempts++
	attempt.PipelineRunID = pr.ID
	_ = o.store.UpdateAttempt(ctx, attempt)

	status, err := o.pollRun(ctx, pr.ID)
	o.finishAttempt(ctx, &attempt, status)
	return status, err
}

func (o *Orchestrator) finishAttempt(ctx context.Context, attempt *model.RunAttempt, status string) {
	now := o.now()
	attempt.Status = status
	attempt.UpdatedAt = now
	attempt.FinishedAt = &now
	_ = o.store.UpdateAttempt(ctx, *attempt)
}

// pollRun polls a CP pipeline run until it reaches a terminal state or the poll
// timeout/context elapses.
func (o *Orchestrator) pollRun(ctx context.Context, pipelineRunID string) (string, error) {
	deadline := o.now().Add(o.pollTimeout)
	for {
		pr, err := o.cp.GetPipelineRun(ctx, pipelineRunID)
		if err != nil {
			return StatusFailed, err
		}
		switch mapCPStatus(pr.Status) {
		case StatusSuccess:
			return StatusSuccess, nil
		case StatusFailed:
			return StatusFailed, nil
		case StatusCanceled:
			return StatusCanceled, nil
		}
		if o.now().After(deadline) {
			return StatusFailed, fmt.Errorf("orchestrator: poll timeout for run %s", pipelineRunID)
		}
		select {
		case <-ctx.Done():
			return StatusCanceled, ctx.Err()
		case <-time.After(o.pollInterval):
		}
	}
}

// mapCPStatus maps CP run status strings to local statuses.
func mapCPStatus(s string) string {
	switch s {
	case "Succeeded", "Success", "succeeded":
		return StatusSuccess
	case "Failed", "Error", "failed":
		return StatusFailed
	case "Canceled", "Cancelled", "canceled":
		return StatusCanceled
	default:
		return StatusRunning
	}
}

// errLayerFailed signals that a layer had at least one permanent failure.
var errLayerFailed = errors.New("orchestrator: layer failed")

// sortImagesByLayer returns the run's images grouped and ordered base->env->instance.
func sortImagesByLayer(images []model.ImageBuild) map[string][]model.ImageBuild {
	groups := map[string][]model.ImageBuild{}
	for _, img := range images {
		groups[img.Layer] = append(groups[img.Layer], img)
	}
	for layer := range groups {
		sort.Slice(groups[layer], func(i, j int) bool {
			return groups[layer][i].LocalKey < groups[layer][j].LocalKey
		})
	}
	return groups
}
