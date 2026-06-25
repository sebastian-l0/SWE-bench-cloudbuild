// Package store defines the persistence layer for runs, image builds, attempts
// and events. It exposes a Store interface backed by either PostgreSQL (pgx) for
// production or an in-memory implementation for tests and diagnostics.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/model"
)

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("store: not found")

// ErrDuplicate is returned when creating a record whose ID already exists.
var ErrDuplicate = errors.New("store: duplicate id")

func errDuplicate(kind, id string) error {
	return fmt.Errorf("%w: %s %q", ErrDuplicate, kind, id)
}

// Store persists workflow state so a run can be recovered after a backend
// restart. Implementations must be safe for concurrent use.
type Store interface {
	CreateRun(ctx context.Context, run model.Run) error
	GetRun(ctx context.Context, id string) (model.Run, error)
	ListRuns(ctx context.Context) ([]model.Run, error)
	UpdateRun(ctx context.Context, run model.Run) error

	CreateImageBuild(ctx context.Context, img model.ImageBuild) error
	GetImageBuild(ctx context.Context, id string) (model.ImageBuild, error)
	ListImageBuildsByRun(ctx context.Context, runID string) ([]model.ImageBuild, error)
	UpdateImageBuild(ctx context.Context, img model.ImageBuild) error

	CreateAttempt(ctx context.Context, attempt model.RunAttempt) error
	UpdateAttempt(ctx context.Context, attempt model.RunAttempt) error
	ListAttemptsByImage(ctx context.Context, imageBuildID string) ([]model.RunAttempt, error)

	AppendEvent(ctx context.Context, event model.RunEvent) error
	ListEventsByRun(ctx context.Context, runID string, afterID string) ([]model.RunEvent, error)
}
