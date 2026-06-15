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
	store        store.Store
	materializer *materializer.Materializer
	uploader     *tosupload.Uploader
	now          func() time.Time

	mu       sync.Mutex
	cfg      config.Config
	cpClient cp.Client
	orch     *orchestrator.Orchestrator
	running  map[string]context.CancelFunc
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

	s := &Service{
		store:        opts.Store,
		materializer: mat,
		uploader:     up,
		now:          now,
		cfg:          opts.Config,
		cpClient:     cpClient,
		running:      make(map[string]context.CancelFunc),
	}
	s.orch = buildOrchestrator(opts.Config, opts.Store, cpClient, now)
	return s, nil
}

// buildOrchestrator constructs an orchestrator from config and dependencies.
func buildOrchestrator(cfg config.Config, st store.Store, cpClient cp.Client, now func() time.Time) *orchestrator.Orchestrator {
	return orchestrator.New(orchestrator.Options{
		Client: cpClient,
		Store:  st,
		Settings: orchestrator.BuildSettings{
			Registry:  cfg.RegistryNamespace,
			Namespace: cfg.CP.WorkspacePrefix,
			Repo:      cfg.CP.PipelinePrefix,
			TOSBucket: cfg.TOS.Bucket,
			TOSRegion: cfg.VolcTarget,
			TOSPath:   cfg.TOS.ParentPath,
		},
		Concurrency: orchestrator.Concurrency{
			Base:     cfg.Concurrency.Base,
			Env:      cfg.Concurrency.Env,
			Instance: cfg.Concurrency.Instance,
		},
		Now: now,
	})
}

// ConfigUpdate carries UI-entered overrides; empty/nil fields are left unchanged.
type ConfigUpdate struct {
	VolcTarget        *string
	VolcAccessKey     *string
	VolcSecretKey     *string
	TOSBucket         *string
	TOSParentPath     *string
	RegistryNamespace *string
	MockMode          *bool
}

// UpdateConfig applies UI overrides and rebuilds the CP client and orchestrator.
// UI-entered values take precedence over .env, matching the spec.
func (s *Service) UpdateConfig(in ConfigUpdate) (config.Config, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	cfg := s.cfg
	if in.VolcTarget != nil {
		cfg.VolcTarget = *in.VolcTarget
	}
	if in.VolcAccessKey != nil {
		cfg.VolcAccessKey = *in.VolcAccessKey
	}
	if in.VolcSecretKey != nil {
		cfg.VolcSecretKey = *in.VolcSecretKey
	}
	if in.TOSBucket != nil {
		cfg.TOS.Bucket = *in.TOSBucket
	}
	if in.TOSParentPath != nil {
		cfg.TOS.ParentPath = *in.TOSParentPath
	}
	if in.RegistryNamespace != nil {
		cfg.RegistryNamespace = *in.RegistryNamespace
	}
	if in.MockMode != nil {
		cfg.MockMode = *in.MockMode
	}

	cpClient, err := buildCPClient(cfg)
	if err != nil {
		return config.Config{}, err
	}
	s.cfg = cfg
	s.cpClient = cpClient
	s.orch = buildOrchestrator(cfg, s.store, cpClient, s.now)
	return cfg, nil
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

// snapshot returns the current config and orchestrator under lock.
func (s *Service) snapshot() (config.Config, *orchestrator.Orchestrator) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cfg, s.orch
}

// CreateRunInput configures a new run.
type CreateRunInput struct {
	Name      string
	OutputDir string // generated-directory mode input
	Dataset   string
}

// CreateRun persists a new pending run.
func (s *Service) CreateRun(ctx context.Context, in CreateRunInput) (model.Run, error) {
	cfg, _ := s.snapshot()
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = "run-" + s.now().UTC().Format("20060102150405")
	}
	dataset := in.Dataset
	if dataset == "" {
		dataset = cfg.Dataset.Name
	}
	run := model.Run{
		ID:        newID(),
		Name:      name,
		Status:    orchestrator.StatusPending,
		Phase:     "created",
		Dataset:   dataset,
		TOSBucket: cfg.TOS.Bucket,
		Registry:  cfg.RegistryNamespace,
		CreatedAt: s.now().UTC(),
	}
	if err := s.store.CreateRun(ctx, run); err != nil {
		return model.Run{}, err
	}
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
	cfg, orch := s.snapshot()
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
			Split:       cfg.Dataset.Split,
			ImagePrefix: cfg.RegistryNamespace,
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
	if !cfg.MockMode && cfg.TOS.Bucket != "" {
		upRes, err := s.uploader.Upload(ctx, outDir, cfg.TOS.Bucket, cfg.TOS.ParentPath, s.now())
		if err != nil {
			s.markRunFailed(ctx, run, "upload", err)
			return
		}
		run.TOSPrefix = upRes.Prefix
		_ = s.store.UpdateRun(ctx, run)
	}

	run.ManifestJSON = m.RawJSON
	_ = s.store.UpdateRun(ctx, run)

	_ = orch.Build(ctx, run, m)
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
	orch := s.orch
	s.mu.Unlock()
	return orch.CancelRun(ctx, runID)
}

// RetryImage retries a single failed image.
func (s *Service) RetryImage(ctx context.Context, imageID string) error {
	_, orch := s.snapshot()
	return orch.RetryImage(ctx, imageID)
}

// ImageLog returns redacted log content for an image.
func (s *Service) ImageLog(ctx context.Context, imageID string) (string, error) {
	_, orch := s.snapshot()
	raw, err := orch.ImageLog(ctx, imageID)
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
	cfg, _ := s.snapshot()
	return cfg
}
