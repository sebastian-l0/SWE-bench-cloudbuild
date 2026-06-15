package cp

import "context"

// Workspace is a CP workspace.
type Workspace struct {
	ID   string `json:"Id"`
	Name string `json:"Name"`
}

// ServiceConnection is a CP service connection (e.g. a registry or code source).
type ServiceConnection struct {
	ID   string `json:"Id"`
	Name string `json:"Name"`
	Type string `json:"Type"`
}

// Pipeline is a CP pipeline.
type Pipeline struct {
	ID          string `json:"Id"`
	Name        string `json:"Name"`
	WorkspaceID string `json:"WorkspaceId"`
}

// PipelineRun is a single execution of a pipeline.
type PipelineRun struct {
	ID         string `json:"Id"`
	PipelineID string `json:"PipelineId"`
	Status     string `json:"Status"`
}

// TaskRun is a task within a pipeline run.
type TaskRun struct {
	ID     string `json:"Id"`
	Name   string `json:"Name"`
	Status string `json:"Status"`
}

// LogPage is a page of task-run logs.
type LogPage struct {
	Content   string `json:"Content"`
	NextToken string `json:"NextToken"`
}

// CreateWorkspaceInput is the input for creating a workspace.
type CreateWorkspaceInput struct {
	Name string `json:"Name"`
}

// CreateServiceConnectionInput is the input for creating a service connection.
type CreateServiceConnectionInput struct {
	Name string `json:"Name"`
	Type string `json:"Type"`
}

// CreatePipelineInput is the input for creating a pipeline. The CP create-pipeline
// contract is unstable; callers should treat this as best-effort.
type CreatePipelineInput struct {
	WorkspaceID string `json:"WorkspaceId"`
	Name        string `json:"Name"`
	// YAML holds the pipeline definition. Field names follow CP conventions and
	// may change.
	YAML string `json:"Yaml,omitempty"`
}

// RunPipelineInput triggers a pipeline run with variables.
type RunPipelineInput struct {
	PipelineID string            `json:"PipelineId"`
	Variables  map[string]string `json:"Variables,omitempty"`
}

// Client is the CP API surface used by the orchestrator. Both the real HTTP
// client and the in-memory mock implement it.
type Client interface {
	CreateWorkspace(ctx context.Context, in CreateWorkspaceInput) (Workspace, error)
	ListWorkspaces(ctx context.Context) ([]Workspace, error)
	DeleteWorkspace(ctx context.Context, id string) error

	CreateServiceConnection(ctx context.Context, in CreateServiceConnectionInput) (ServiceConnection, error)
	GetServiceConnection(ctx context.Context, id string) (ServiceConnection, error)
	ListServiceConnections(ctx context.Context) ([]ServiceConnection, error)
	DeleteServiceConnection(ctx context.Context, id string) error

	CreatePipeline(ctx context.Context, in CreatePipelineInput) (Pipeline, error)
	ListPipelines(ctx context.Context, workspaceID string) ([]Pipeline, error)
	DeletePipeline(ctx context.Context, id string) error

	RunPipeline(ctx context.Context, in RunPipelineInput) (PipelineRun, error)
	GetPipelineRun(ctx context.Context, id string) (PipelineRun, error)
	ListTaskRuns(ctx context.Context, pipelineRunID string) ([]TaskRun, error)
	CancelPipelineRun(ctx context.Context, id string) error

	GetTaskRunLog(ctx context.Context, taskRunID string, nextToken string) (LogPage, error)
}
