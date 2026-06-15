// Package service wires configuration, persistence, the input pipeline and the
// CP orchestrator into high-level operations used by the HTTP API.
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/config"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/manifest"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/materializer"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/model"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/orchestrator"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/store"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/tosupload"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/volc/cp"
)

// newID returns a random hex identifier.
func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// Service coordinates a run from materialization through CP build.
type Service struct {
	cfg          config.Config
	store        store.Store
	cpClient     cp.Client
	materializer *materializer.Materializer
	uploader     *tosupload.Uploader
	orch         *orchestrator.Orchestrator
	now          func() time.Time

	mu      sync.Mutex
	running map[string]context.CancelFunc
}

// Options configures a Service. Materializer/Uploader may be nil to use defaults.
type Options struct {
	Config       config.Config
	Store        store.Store
	CPClient     cp.Client
	Materializer *materializer.Materializer
	Uploader     *tosupload.Uploader
	Now          func() time.Time
}

// New builds a Service. When CPClient is nil it is selected from config (mock or
// real). Materializer/Uploader default to real implementations.
func New(opts Options) (*Service, error) {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	cpClient := opts.CPClient
	if cpClient == nil {
		var err error
		cpClient, err = buildCPClient(opts.Config)
		if err != nil {
			return nil, err
		}
	}
	mat := opts.Materializer
	if mat == nil {
		mat = materializer.New(nil)
	}
	up := opts.Uploader
	if up == nil {
		up = tosupload.New(nil, "")
	}

	orch := orchestrator.New(orchestrator.Options{
		Client: cpClient,
		Store:  opts.Store,
		Settings: orchestrator.BuildSettings{
			Registry:  opts.Config.RegistryNamespace,
			Namespace: opts.Config.CP.WorkspacePrefix,
			Repo:      opts.Config.CP.PipelinePrefix,
			TOSBucket: opts.Config.TOS.Bucket,
			TOSRegion: opts.Config.VolcTarget,
			TOSPath:   opts.Config.TOS.ParentPath,
		},
		Concurrency: orchestrator.Concurrency{
			Base:     opts.Config.Concurrency.Base,
			Env:      opts.Config.Concurrency.Env,
			Instance: opts.Config.Concurrency.Instance,
		},
		Now: now,
	})

	return &Service{
		cfg:          opts.Config,
		store:        opts.Store,
		cpClient:     cpClient,
		materializer: mat,
		uploader:     up,
		orch:         orch,
		now:          now,
		running:      make(map[string]context.CancelFunc),
	}, nil
}

func buildCPClient(cfg config.Config) (cp.Client, error) {
	if cfg.MockMode {
		return cp.NewMockClient(), nil
	}
	target, err := cp.ResolveTarget(cfg.VolcTarget)
	if err != nil {
		return nil, err
	}
	return cp.NewHTTPClient(target, cp.Credentials{
		AccessKey: cfg.VolcAccessKey,
		SecretKey: cfg.VolcSecretKey,
	}), nil
}

// CreateRunInput configures a new run.
type CreateRunInput struct {
	Name      string
	OutputDir string // generated-directory mode input
	Dataset   string
}

// CreateRun persists a new pending run.
func (s *Service) CreateRun(ctx context.Context, in CreateRunInput) (model.Run, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = "run-" + s.now().UTC().Format("20060102150405")
	}
	dataset := in.Dataset
	if dataset == "" {
		dataset = s.cfg.Dataset.Name
	}
	run := model.Run{
		ID:        newID(),
		Name:      name,
		Status:    orchestrator.StatusPending,
		Phase:     "created",
		Dataset:   dataset,
		TOSBucket: s.cfg.TOS.Bucket,
		Registry:  s.cfg.RegistryNamespace,
		CreatedAt: s.now().UTC(),
	}
	if err := s.store.CreateRun(ctx, run); err != nil {
		return model.Run{}, err
	}
	// Stash the output dir in the run error field? No—use a dedicated approach:
	// persist via metadata is out of scope; carry it through StartRun argument.
	s.setOutputDir(run.ID, in.OutputDir)
	return run, nil
}

// outputDirs tracks generated-directory inputs per run for the current process.
var outputDirs sync.Map

func (s *Service) setOutputDir(runID, dir string) {
	if dir != "" {
		outputDirs.Store(runID, dir)
	}
}

func (s *Service) outputDir(runID string) string {
	if v, ok := outputDirs.Load(runID); ok {
		return v.(string)
	}
	return ""
}

// StartRun launches the full workflow in the background: materialize -> upload
// -> CP build. It returns immediately after marking the run running.
func (s *Service) StartRun(runID string) error {
	run, err := s.store.GetRun(context.Background(), runID)
	if err != nil {
		return err
	}
	if run.Status == orchestrator.StatusRunning {
		return fmt.Errorf("service: run already running")
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.running[runID] = cancel
	s.mu.Unlock()

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.running, runID)
			s.mu.Unlock()
			cancel()
		}()
		s.execute(ctx, run)
	}()
	return nil
}

// execute runs materialize -> upload -> build, persisting failures on the run.
func (s *Service) execute(ctx context.Context, run model.Run) {
	outDir := s.outputDir(run.ID)

	var m *manifest.Manifest
	if outDir != "" {
		res, err := s.materializer.Materialize(ctx, materializer.Options{
			Mode:      materializer.ModeGeneratedDir,
			OutputDir: outDir,
		})
		if err != nil {
			s.markRunFailed(ctx, run, "materialize", err)
			return
		}
		m = res.Manifest
	} else {
		res, err := s.materializer.Materialize(ctx, materializer.Options{
			Mode:        materializer.ModeCommand,
			OutputDir:   defaultOutputDir(run.ID),
			Dataset:     run.Dataset,
			Split:       s.cfg.Dataset.Split,
			ImagePrefix: s.cfg.RegistryNamespace,
			Tag:         "latest",
		})
		if err != nil {
			s.markRunFailed(ctx, run, "materialize", err)
			return
		}
		m = res.Manifest
		outDir = res.OutputDir
	}

	// Upload (skipped in mock mode or when bucket is unset).
	if !s.cfg.MockMode && s.cfg.TOS.Bucket != "" {
		upRes, err := s.uploader.Upload(ctx, outDir, s.cfg.TOS.Bucket, s.cfg.TOS.ParentPath, s.now())
		if err != nil {
			s.markRunFailed(ctx, run, "upload", err)
			return
		}
		run.TOSPrefix = upRes.Prefix
		_ = s.store.UpdateRun(ctx, run)
	}

	run.ManifestJSON = m.RawJSON
	_ = s.store.UpdateRun(ctx, run)

	_ = s.orch.Build(ctx, run, m)
}

func (s *Service) markRunFailed(ctx context.Context, run model.Run, phase string, err error) {
	now := s.now().UTC()
	run.Status = orchestrator.StatusFailed
	run.Phase = phase
	run.Error = config.Redact(err.Error())
	run.FinishedAt = &now
	_ = s.store.UpdateRun(ctx, run)
}

func defaultOutputDir(runID string) string {
	return "/tmp/swe-cloudbuild/" + runID
}

// CancelRun cancels an active run.
func (s *Service) CancelRun(ctx context.Context, runID string) error {
	s.mu.Lock()
	if cancel, ok := s.running[runID]; ok {
		cancel()
	}
	s.mu.Unlock()
	return s.orch.CancelRun(ctx, runID)
}

// RetryImage retries a single failed image.
func (s *Service) RetryImage(ctx context.Context, imageID string) error {
	return s.orch.RetryImage(ctx, imageID)
}

// ImageLog returns redacted log content for an image.
func (s *Service) ImageLog(ctx context.Context, imageID string) (string, error) {
	raw, err := s.orch.ImageLog(ctx, imageID)
	if err != nil {
		return "", err
	}
	return config.Redact(raw), nil
}

// ListRuns returns all runs.
func (s *Service) ListRuns(ctx context.Context) ([]model.Run, error) {
	return s.store.ListRuns(ctx)
}

// GetRun returns a run and its image builds.
func (s *Service) GetRun(ctx context.Context, runID string) (model.Run, []model.ImageBuild, error) {
	run, err := s.store.GetRun(ctx, runID)
	if err != nil {
		return model.Run{}, nil, err
	}
	images, err := s.store.ListImageBuildsByRun(ctx, runID)
	if err != nil {
		return model.Run{}, nil, err
	}
	return run, images, nil
}

// GetImage returns a single image build.
func (s *Service) GetImage(ctx context.Context, imageID string) (model.ImageBuild, error) {
	return s.store.GetImageBuild(ctx, imageID)
}

// Events returns persisted run events after the given event ID (for SSE resume).
func (s *Service) Events(ctx context.Context, runID, afterID string) ([]model.RunEvent, error) {
	return s.store.ListEventsByRun(ctx, runID, afterID)
}

// Config returns the effective configuration.
func (s *Service) Config() config.Config {
	return s.cfg
}
