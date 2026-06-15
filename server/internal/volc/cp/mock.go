package cp

import (
	"context"
	"fmt"
	"sync"
)

// MockClient is an in-memory CP Client implementation for local demos and tests.
// It assigns deterministic-ish IDs and tracks object lifecycle without making
// network calls.
type MockClient struct {
	mu          sync.Mutex
	seq         int
	workspaces  map[string]Workspace
	connections map[string]ServiceConnection
	pipelines   map[string]Pipeline
	runs        map[string]PipelineRun
	taskRuns    map[string][]TaskRun // keyed by pipeline run ID
}

// NewMockClient returns an empty in-memory CP client.
func NewMockClient() *MockClient {
	return &MockClient{
		workspaces:  make(map[string]Workspace),
		connections: make(map[string]ServiceConnection),
		pipelines:   make(map[string]Pipeline),
		runs:        make(map[string]PipelineRun),
		taskRuns:    make(map[string][]TaskRun),
	}
}

var _ Client = (*MockClient)(nil)

func (m *MockClient) nextID(prefix string) string {
	m.seq++
	return fmt.Sprintf("%s-%d", prefix, m.seq)
}

func notFound(kind, id string) error {
	return &APIError{HTTPStatus: 404, Code: "NotFound", Message: fmt.Sprintf("%s %q not found", kind, id)}
}

func (m *MockClient) CreateWorkspace(_ context.Context, in CreateWorkspaceInput) (Workspace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ws := Workspace{ID: m.nextID("ws"), Name: in.Name}
	m.workspaces[ws.ID] = ws
	return ws, nil
}

func (m *MockClient) ListWorkspaces(_ context.Context) ([]Workspace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Workspace, 0, len(m.workspaces))
	for _, ws := range m.workspaces {
		out = append(out, ws)
	}
	return out, nil
}

func (m *MockClient) DeleteWorkspace(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.workspaces[id]; !ok {
		return notFound("workspace", id)
	}
	delete(m.workspaces, id)
	return nil
}

func (m *MockClient) CreateServiceConnection(_ context.Context, in CreateServiceConnectionInput) (ServiceConnection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sc := ServiceConnection{ID: m.nextID("sc"), Name: in.Name, Type: in.Type}
	m.connections[sc.ID] = sc
	return sc, nil
}

func (m *MockClient) GetServiceConnection(_ context.Context, id string) (ServiceConnection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	sc, ok := m.connections[id]
	if !ok {
		return ServiceConnection{}, notFound("service connection", id)
	}
	return sc, nil
}

func (m *MockClient) ListServiceConnections(_ context.Context) ([]ServiceConnection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ServiceConnection, 0, len(m.connections))
	for _, sc := range m.connections {
		out = append(out, sc)
	}
	return out, nil
}

func (m *MockClient) DeleteServiceConnection(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.connections[id]; !ok {
		return notFound("service connection", id)
	}
	delete(m.connections, id)
	return nil
}

func (m *MockClient) CreatePipeline(_ context.Context, in CreatePipelineInput) (Pipeline, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	p := Pipeline{ID: m.nextID("pl"), Name: in.Name, WorkspaceID: in.WorkspaceID}
	m.pipelines[p.ID] = p
	return p, nil
}

func (m *MockClient) ListPipelines(_ context.Context, workspaceID string) ([]Pipeline, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Pipeline, 0)
	for _, p := range m.pipelines {
		if workspaceID == "" || p.WorkspaceID == workspaceID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (m *MockClient) DeletePipeline(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.pipelines[id]; !ok {
		return notFound("pipeline", id)
	}
	delete(m.pipelines, id)
	return nil
}

func (m *MockClient) RunPipeline(_ context.Context, in RunPipelineInput) (PipelineRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.pipelines[in.PipelineID]; !ok {
		return PipelineRun{}, notFound("pipeline", in.PipelineID)
	}
	run := PipelineRun{ID: m.nextID("run"), PipelineID: in.PipelineID, Status: "Running"}
	m.runs[run.ID] = run
	m.taskRuns[run.ID] = []TaskRun{{ID: m.nextID("task"), Name: "build", Status: "Running"}}
	return run, nil
}

// GetPipelineRun returns the run. In the mock, a run transitions to Succeeded on
// the first read after creation to allow simple end-to-end progression.
func (m *MockClient) GetPipelineRun(_ context.Context, id string) (PipelineRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.runs[id]
	if !ok {
		return PipelineRun{}, notFound("pipeline run", id)
	}
	if run.Status == "Running" {
		run.Status = "Succeeded"
		m.runs[id] = run
		if tasks, ok := m.taskRuns[id]; ok {
			for i := range tasks {
				tasks[i].Status = "Succeeded"
			}
			m.taskRuns[id] = tasks
		}
	}
	return run, nil
}

func (m *MockClient) ListTaskRuns(_ context.Context, pipelineRunID string) ([]TaskRun, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	tasks, ok := m.taskRuns[pipelineRunID]
	if !ok {
		return nil, notFound("pipeline run", pipelineRunID)
	}
	out := make([]TaskRun, len(tasks))
	copy(out, tasks)
	return out, nil
}

func (m *MockClient) CancelPipelineRun(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, ok := m.runs[id]
	if !ok {
		return notFound("pipeline run", id)
	}
	run.Status = "Canceled"
	m.runs[id] = run
	return nil
}

func (m *MockClient) GetTaskRunLog(_ context.Context, taskRunID string, _ string) (LogPage, error) {
	return LogPage{Content: fmt.Sprintf("mock log for task %s\n", taskRunID)}, nil
}
