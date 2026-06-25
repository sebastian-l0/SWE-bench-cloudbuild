package store

import (
	"context"
	"sort"
	"sync"

	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/model"
)

// MemoryStore is an in-memory Store implementation for tests and diagnostics.
// It deep-copies records on read and write so callers cannot mutate stored state
// by holding references.
type MemoryStore struct {
	mu       sync.RWMutex
	runs     map[string]model.Run
	images   map[string]model.ImageBuild
	attempts map[string]model.RunAttempt
	events   []model.RunEvent
}

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		runs:     make(map[string]model.Run),
		images:   make(map[string]model.ImageBuild),
		attempts: make(map[string]model.RunAttempt),
	}
}

var _ Store = (*MemoryStore)(nil)

func (s *MemoryStore) CreateRun(_ context.Context, run model.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.runs[run.ID]; ok {
		return errDuplicate("run", run.ID)
	}
	s.runs[run.ID] = run
	return nil
}

func (s *MemoryStore) GetRun(_ context.Context, id string) (model.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[id]
	if !ok {
		return model.Run{}, ErrNotFound
	}
	return run, nil
}

func (s *MemoryStore) ListRuns(_ context.Context) ([]model.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.Run, 0, len(s.runs))
	for _, run := range s.runs {
		out = append(out, run)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	return out, nil
}

func (s *MemoryStore) UpdateRun(_ context.Context, run model.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.runs[run.ID]; !ok {
		return ErrNotFound
	}
	s.runs[run.ID] = run
	return nil
}

func (s *MemoryStore) CreateImageBuild(_ context.Context, img model.ImageBuild) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.images[img.ID]; ok {
		return errDuplicate("image_build", img.ID)
	}
	s.images[img.ID] = img
	return nil
}

func (s *MemoryStore) GetImageBuild(_ context.Context, id string) (model.ImageBuild, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	img, ok := s.images[id]
	if !ok {
		return model.ImageBuild{}, ErrNotFound
	}
	return img, nil
}

func (s *MemoryStore) ListImageBuildsByRun(_ context.Context, runID string) ([]model.ImageBuild, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.ImageBuild, 0)
	for _, img := range s.images {
		if img.RunID == runID {
			out = append(out, img)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (s *MemoryStore) UpdateImageBuild(_ context.Context, img model.ImageBuild) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.images[img.ID]; !ok {
		return ErrNotFound
	}
	s.images[img.ID] = img
	return nil
}

func (s *MemoryStore) CreateAttempt(_ context.Context, attempt model.RunAttempt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.attempts[attempt.ID]; ok {
		return errDuplicate("run_attempt", attempt.ID)
	}
	s.attempts[attempt.ID] = attempt
	return nil
}

func (s *MemoryStore) UpdateAttempt(_ context.Context, attempt model.RunAttempt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.attempts[attempt.ID]; !ok {
		return ErrNotFound
	}
	s.attempts[attempt.ID] = attempt
	return nil
}

func (s *MemoryStore) ListAttemptsByImage(_ context.Context, imageBuildID string) ([]model.RunAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.RunAttempt, 0)
	for _, attempt := range s.attempts {
		if attempt.ImageBuildID == imageBuildID {
			out = append(out, attempt)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].StartedAt.Equal(out[j].StartedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].StartedAt.Before(out[j].StartedAt)
	})
	return out, nil
}

func (s *MemoryStore) AppendEvent(_ context.Context, event model.RunEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

// ListEventsByRun returns events for a run ordered by creation. When afterID is
// non-empty, only events recorded after that event ID are returned, supporting
// SSE resume.
func (s *MemoryStore) ListEventsByRun(_ context.Context, runID string, afterID string) ([]model.RunEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.RunEvent, 0)
	include := afterID == ""
	for _, event := range s.events {
		if event.RunID != runID {
			continue
		}
		if !include {
			if event.ID == afterID {
				include = true
			}
			continue
		}
		out = append(out, event)
	}
	return out, nil
}
