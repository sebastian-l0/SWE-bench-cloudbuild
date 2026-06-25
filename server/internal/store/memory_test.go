package store

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/model"
)

func TestMemoryStoreRunLifecycle(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	now := time.Now().UTC()
	run := model.Run{ID: "run-1", Name: "demo", Status: "pending", Phase: "materializing", CreatedAt: now}
	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if err := s.CreateRun(ctx, run); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate CreateRun err = %v, want ErrDuplicate", err)
	}

	got, err := s.GetRun(ctx, "run-1")
	if err != nil || got.Name != "demo" {
		t.Fatalf("GetRun = %#v, %v", got, err)
	}

	run.Status = "running"
	if err := s.UpdateRun(ctx, run); err != nil {
		t.Fatalf("UpdateRun: %v", err)
	}
	got, _ = s.GetRun(ctx, "run-1")
	if got.Status != "running" {
		t.Fatalf("status = %q, want running", got.Status)
	}

	if err := s.UpdateRun(ctx, model.Run{ID: "missing"}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("UpdateRun missing err = %v, want ErrNotFound", err)
	}
	if _, err := s.GetRun(ctx, "missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetRun missing err = %v, want ErrNotFound", err)
	}
}

func TestMemoryStoreListRunsOrderedByCreatedDesc(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	base := time.Now().UTC()
	_ = s.CreateRun(ctx, model.Run{ID: "old", CreatedAt: base})
	_ = s.CreateRun(ctx, model.Run{ID: "new", CreatedAt: base.Add(time.Minute)})

	runs, err := s.ListRuns(ctx)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) != 2 || runs[0].ID != "new" || runs[1].ID != "old" {
		t.Fatalf("order = %v", []string{runs[0].ID, runs[1].ID})
	}
}

func TestMemoryStoreImageBuildsScopedByRun(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	base := time.Now().UTC()
	_ = s.CreateImageBuild(ctx, model.ImageBuild{ID: "b1", RunID: "r1", Layer: "base", CreatedAt: base})
	_ = s.CreateImageBuild(ctx, model.ImageBuild{ID: "e1", RunID: "r1", Layer: "env", CreatedAt: base.Add(time.Second)})
	_ = s.CreateImageBuild(ctx, model.ImageBuild{ID: "x1", RunID: "r2", Layer: "base", CreatedAt: base})

	imgs, err := s.ListImageBuildsByRun(ctx, "r1")
	if err != nil {
		t.Fatalf("ListImageBuildsByRun: %v", err)
	}
	if len(imgs) != 2 || imgs[0].ID != "b1" || imgs[1].ID != "e1" {
		t.Fatalf("got %v", imgs)
	}

	imgs[0].Status = "success"
	if err := s.UpdateImageBuild(ctx, imgs[0]); err != nil {
		t.Fatalf("UpdateImageBuild: %v", err)
	}
	got, _ := s.GetImageBuild(ctx, "b1")
	if got.Status != "success" {
		t.Fatalf("status = %q", got.Status)
	}
}

func TestMemoryStoreAttempts(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	base := time.Now().UTC()
	_ = s.CreateAttempt(ctx, model.RunAttempt{ID: "a1", ImageBuildID: "img", Status: "running", StartedAt: base})
	_ = s.CreateAttempt(ctx, model.RunAttempt{ID: "a2", ImageBuildID: "img", Status: "running", StartedAt: base.Add(time.Second)})

	attempts, err := s.ListAttemptsByImage(ctx, "img")
	if err != nil || len(attempts) != 2 || attempts[0].ID != "a1" {
		t.Fatalf("attempts = %v, err = %v", attempts, err)
	}

	attempts[1].Status = "success"
	if err := s.UpdateAttempt(ctx, attempts[1]); err != nil {
		t.Fatalf("UpdateAttempt: %v", err)
	}
	got, _ := s.ListAttemptsByImage(ctx, "img")
	if got[1].Status != "success" {
		t.Fatalf("status = %q", got[1].Status)
	}
}

func TestMemoryStoreEventsResumeAfterID(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()
	base := time.Now().UTC()
	for i, id := range []string{"ev1", "ev2", "ev3"} {
		_ = s.AppendEvent(ctx, model.RunEvent{ID: id, RunID: "r1", Type: "phase", CreatedAt: base.Add(time.Duration(i) * time.Second)})
	}
	_ = s.AppendEvent(ctx, model.RunEvent{ID: "other", RunID: "r2", Type: "phase", CreatedAt: base})

	all, err := s.ListEventsByRun(ctx, "r1", "")
	if err != nil || len(all) != 3 {
		t.Fatalf("all events = %v, err = %v", all, err)
	}

	after, err := s.ListEventsByRun(ctx, "r1", "ev1")
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if len(after) != 2 || after[0].ID != "ev2" || after[1].ID != "ev3" {
		t.Fatalf("resume events = %v", after)
	}
}

// memoryStoreSatisfiesInterface ensures the in-memory store can substitute for
// the Postgres store in dependent packages.
func TestMemoryStoreSatisfiesInterface(t *testing.T) {
	var _ Store = NewMemoryStore()
}
