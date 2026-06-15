package orchestrator

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/manifest"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/model"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/store"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/volc/cp"
)

// programmableClient is a cp.Client whose run outcomes are controlled per tag.
type programmableClient struct {
	cp.Client
	// failTags maps a run's tag (local key) to a terminal status to report.
	failTags map[string]string
	runTag   map[string]string // pipelineRunID -> tag
	canceled map[string]bool
	seq      int
}

func newProgrammableClient() *programmableClient {
	return &programmableClient{
		Client:   cp.NewMockClient(),
		failTags: map[string]string{},
		runTag:   map[string]string{},
		canceled: map[string]bool{},
	}
}

func (p *programmableClient) RunPipeline(ctx context.Context, in cp.RunPipelineInput) (cp.PipelineRun, error) {
	pr, err := p.Client.RunPipeline(ctx, in)
	if err != nil {
		return pr, err
	}
	for _, param := range in.Params {
		if param.Key == "tag" {
			p.runTag[pr.ID] = param.Value
		}
	}
	return pr, nil
}

func (p *programmableClient) GetPipelineRun(ctx context.Context, workspaceID, pipelineID, runID string) (cp.PipelineRun, error) {
	if p.canceled[runID] {
		return cp.PipelineRun{ID: runID, Status: "Canceled"}, nil
	}
	tag := p.runTag[runID]
	if status, ok := p.failTags[tag]; ok {
		return cp.PipelineRun{ID: runID, Status: status}, nil
	}
	return cp.PipelineRun{ID: runID, Status: "Succeeded",
		Stages: []cp.RunStage{{ID: "s", Name: "build", Status: "Succeeded",
			Tasks: []cp.RunTask{{ID: "t", Name: "build", Status: "Succeeded"}}}}}, nil
}

func (p *programmableClient) CancelPipelineRun(ctx context.Context, workspaceID, pipelineID, id string) error {
	p.canceled[id] = true
	return nil
}

func testOrchestrator(t *testing.T, client cp.Client) (*Orchestrator, store.Store) {
	t.Helper()
	st := store.NewMemoryStore()
	seq := 0
	o := New(Options{
		Client:       client,
		Store:        st,
		Settings:     BuildSettings{Registry: "reg", Namespace: "ns", Repo: "repo", TOSBucket: "b", TOSRegion: "cn-beijing", TOSPath: "p/20260615"},
		Concurrency:  Concurrency{Base: 1, Env: 2, Instance: 2},
		IDGen:        func() string { seq++; return "id-" + itoa(seq) },
		Now:          func() time.Time { return time.Unix(0, 0).UTC() },
		PollInterval: time.Millisecond,
		PollTimeout:  time.Second,
	})
	return o, st
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

func threeLayerManifest() *manifest.Manifest {
	return &manifest.Manifest{
		Images: []manifest.Image{
			{Layer: manifest.LayerBase, LocalKey: "base1", TargetImage: "reg:base1", ContextPath: "contexts/base/b", Dockerfile: "Dockerfile"},
			{Layer: manifest.LayerEnv, LocalKey: "env1", TargetImage: "reg:env1", ContextPath: "contexts/env/e", Dockerfile: "Dockerfile", DependsOnKey: "base1"},
			{Layer: manifest.LayerInstance, LocalKey: "inst1", TargetImage: "reg:inst1", ContextPath: "contexts/instances/i", Dockerfile: "Dockerfile", DependsOnKey: "env1"},
		},
	}
}

func newRun(id string) model.Run {
	return model.Run{ID: id, Name: "demo", Status: StatusPending, Phase: "uploading"}
}

func TestBuildStrictSuccess(t *testing.T) {
	o, st := testOrchestrator(t, newProgrammableClient())
	run := newRun("r1")
	if err := st.CreateRun(context.Background(), run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := o.Build(context.Background(), run, threeLayerManifest(), ""); err != nil {
		t.Fatalf("Build: %v", err)
	}

	got, _ := st.GetRun(context.Background(), "r1")
	if got.Status != StatusSuccess {
		t.Fatalf("run status = %q", got.Status)
	}
	imgs, _ := st.ListImageBuildsByRun(context.Background(), "r1")
	if len(imgs) != 3 {
		t.Fatalf("images = %d", len(imgs))
	}
	for _, img := range imgs {
		if img.Status != StatusSuccess {
			t.Fatalf("image %s status = %q", img.LocalKey, img.Status)
		}
		if img.PipelineID == "" || img.WorkspaceID == "" {
			t.Fatalf("image %s missing cp ids: %#v", img.LocalKey, img)
		}
	}
}

func TestBuildGateBlocksDownstreamOnFailure(t *testing.T) {
	client := newProgrammableClient()
	client.failTags["base1"] = "Failed"
	o, st := testOrchestrator(t, client)
	run := newRun("r2")
	_ = st.CreateRun(context.Background(), run)

	err := o.Build(context.Background(), run, threeLayerManifest(), "")
	if !errors.Is(err, errLayerFailed) {
		t.Fatalf("Build err = %v, want errLayerFailed", err)
	}

	got, _ := st.GetRun(context.Background(), "r2")
	if got.Status != StatusFailed {
		t.Fatalf("run status = %q", got.Status)
	}
	imgs, _ := st.ListImageBuildsByRun(context.Background(), "r2")
	byKey := map[string]string{}
	for _, img := range imgs {
		byKey[img.LocalKey] = img.Status
	}
	if byKey["base1"] != StatusFailed {
		t.Fatalf("base1 = %q, want failed", byKey["base1"])
	}
	if byKey["env1"] != StatusSkipped || byKey["inst1"] != StatusSkipped {
		t.Fatalf("downstream not skipped: env1=%q inst1=%q", byKey["env1"], byKey["inst1"])
	}
}

func TestBuildPrepareResourcesFailureFailsRun(t *testing.T) {
	client := &failingPrepareClient{Client: cp.NewMockClient()}
	o, st := testOrchestrator(t, client)
	run := newRun("r3")
	_ = st.CreateRun(context.Background(), run)

	err := o.Build(context.Background(), run, threeLayerManifest(), "")
	if err == nil {
		t.Fatal("expected prepare failure")
	}
	got, _ := st.GetRun(context.Background(), "r3")
	if got.Status != StatusFailed {
		t.Fatalf("run status = %q", got.Status)
	}
	imgs, _ := st.ListImageBuildsByRun(context.Background(), "r3")
	if len(imgs) != 0 {
		t.Fatalf("no image builds expected when prepare fails, got %d", len(imgs))
	}
}

type failingPrepareClient struct {
	cp.Client
}

func (f *failingPrepareClient) CreateWorkspace(context.Context, cp.CreateWorkspaceInput) (cp.Workspace, error) {
	return cp.Workspace{}, errors.New("quota exceeded")
}

func TestRetryImageReRuns(t *testing.T) {
	client := newProgrammableClient()
	client.failTags["env1"] = "Failed"
	o, st := testOrchestrator(t, client)
	run := newRun("r4")
	_ = st.CreateRun(context.Background(), run)
	_ = o.Build(context.Background(), run, threeLayerManifest(), "")

	imgs, _ := st.ListImageBuildsByRun(context.Background(), "r4")
	var envID string
	for _, img := range imgs {
		if img.LocalKey == "env1" {
			envID = img.ID
		}
	}
	if envID == "" {
		t.Fatal("env1 not found")
	}
	// Clear the failure and retry.
	delete(client.failTags, "env1")
	if err := o.RetryImage(context.Background(), envID); err != nil {
		t.Fatalf("RetryImage: %v", err)
	}
	got, _ := st.GetImageBuild(context.Background(), envID)
	if got.Status != StatusSuccess {
		t.Fatalf("retried image status = %q", got.Status)
	}
	if got.Attempts < 2 {
		t.Fatalf("attempts = %d, want >=2", got.Attempts)
	}
	attempts, _ := st.ListAttemptsByImage(context.Background(), envID)
	if len(attempts) < 2 {
		t.Fatalf("attempt records = %d, want >=2", len(attempts))
	}
}

func TestImageLogReturnsContent(t *testing.T) {
	o, st := testOrchestrator(t, newProgrammableClient())
	run := newRun("r5")
	_ = st.CreateRun(context.Background(), run)
	_ = o.Build(context.Background(), run, threeLayerManifest(), "")

	imgs, _ := st.ListImageBuildsByRun(context.Background(), "r5")
	log, err := o.ImageLog(context.Background(), imgs[0].ID)
	if err != nil {
		t.Fatalf("ImageLog: %v", err)
	}
	if log == "" {
		t.Fatal("expected log content")
	}
}

func TestCancelRunMarksImagesCanceled(t *testing.T) {
	o, st := testOrchestrator(t, newProgrammableClient())
	run := newRun("r6")
	_ = st.CreateRun(context.Background(), run)
	// Seed a running image build with a CP run id.
	now := time.Unix(0, 0).UTC()
	img := model.ImageBuild{ID: "img-c", RunID: "r6", Layer: manifest.LayerBase, LocalKey: "base1",
		Status: StatusRunning, LastRunID: "run-x", CreatedAt: now, UpdatedAt: now}
	if err := st.CreateImageBuild(context.Background(), img); err != nil {
		t.Fatalf("CreateImageBuild: %v", err)
	}

	if err := o.CancelRun(context.Background(), "r6"); err != nil {
		t.Fatalf("CancelRun: %v", err)
	}
	gotRun, _ := st.GetRun(context.Background(), "r6")
	if gotRun.Status != StatusCanceled {
		t.Fatalf("run status = %q", gotRun.Status)
	}
	gotImg, _ := st.GetImageBuild(context.Background(), "img-c")
	if gotImg.Status != StatusCanceled {
		t.Fatalf("image status = %q", gotImg.Status)
	}
}

func TestRunParamsMapping(t *testing.T) {
	s := BuildSettings{Registry: "reg", Namespace: "ns", Repo: "repo", TOSBucket: "bkt", TOSRegion: "cn-beijing", TOSPath: "p/ts"}
	params := runParams(manifest.LayerEnv, "reg.example.com/ns/repo:sweb.env.py.x86_64.abc", s)
	got := map[string]string{}
	for _, p := range params {
		got[p.Key] = p.Value
	}
	if got["type"] != "env" || got["script"] != "setup_env.sh" {
		t.Fatalf("env params = %#v", got)
	}
	// tag is the registry tag (segment after the last colon of the target image).
	if got["tag"] != "sweb.env.py.x86_64.abc" {
		t.Fatalf("tag = %q", got["tag"])
	}
	if got["registry"] != "reg" || got["namespace"] != "ns" || got["repo"] != "repo" {
		t.Fatalf("registry params = %#v", got)
	}
	if got["tosbucket"] != "bkt" || got["region"] != "cn-beijing" || got["tospath"] != "p/ts" {
		t.Fatalf("tos params = %#v", got)
	}

	base := runParams(manifest.LayerBase, "reg:b", s)
	for _, p := range base {
		if p.Key == "script" && p.Value != "none" {
			t.Fatalf("base script = %q, want none", p.Value)
		}
		if p.Key == "type" && p.Value != "base" {
			t.Fatalf("base type = %q", p.Value)
		}
	}
	inst := runParams(manifest.LayerInstance, "reg:i", s)
	for _, p := range inst {
		if p.Key == "script" && p.Value != "setup_repo.sh" {
			t.Fatalf("instance script = %q", p.Value)
		}
		// instance layer must map to the plural "instances" type/dir segment.
		if p.Key == "type" && p.Value != "instances" {
			t.Fatalf("instance type = %q, want instances", p.Value)
		}
	}
}
