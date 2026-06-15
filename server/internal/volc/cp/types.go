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

// PipelineRun is a single execution of a pipeline. Stages/Tasks are populated by
// ListPipelineRuns and carry per-task status.
type PipelineRun struct {
	ID         string     `json:"Id"`
	PipelineID string     `json:"PipelineId"`
	Status     string     `json:"Status"`
	Stages     []RunStage `json:"Stages"`
}

// RunStage is a stage within a pipeline run.
type RunStage struct {
	ID     string    `json:"Id"`
	Name   string    `json:"Name"`
	Status string    `json:"Status"`
	Tasks  []RunTask `json:"Tasks"`
}

// RunTask is a task within a run stage.
type RunTask struct {
	ID     string `json:"Id"`
	Name   string `json:"Name"`
	Status string `json:"Status"`
}

// TaskRun is a task execution returned by ListTaskRuns. Id is the TaskRunId;
// TaskId is the task definition id. Steps carry per-step names used for logs.
type TaskRun struct {
	ID     string     `json:"Id"`
	TaskID string     `json:"TaskId"`
	Name   string     `json:"Name"`
	Status string     `json:"Status"`
	Steps  []TaskStep `json:"Steps"`
}

// TaskStep is a step within a task run.
type TaskStep struct {
	Name      string `json:"Name"`
	Status    string `json:"Status"`
	LogStatus string `json:"LogStatus"`
	LogURI    string `json:"LogUri"`
}

// LogPage is a page of step logs returned by GetTaskRunLog.
type LogPage struct {
	LogLines   []string `json:"LogLines"`
	More       bool     `json:"More"`
	NextOffset int      `json:"NextOffset"`
	NextLimit  int      `json:"NextLimit"`
}

// CreateWorkspaceInput is the input for creating a workspace.
type CreateWorkspaceInput struct {
	Name string `json:"Name"`
	// Visibility is required by CP; common values are "Private" and "Public".
	Visibility string `json:"Visibility"`
}

// CreateServiceConnectionInput is the input for creating a service connection.
type CreateServiceConnectionInput struct {
	Name string `json:"Name"`
	Type string `json:"Type"`
}

// PipelineParameter is a CP pipeline parameter definition.
type PipelineParameter struct {
	Key          string   `json:"Key"`
	Value        string   `json:"Value"`
	Dynamic      bool     `json:"Dynamic"`
	OptionValues []string `json:"OptionValues,omitempty"`
}

// CreatePipelineInput is the input for creating a pipeline. The CP create-pipeline
// contract is unstable; callers should treat this as best-effort.
type CreatePipelineInput struct {
	WorkspaceID string `json:"WorkspaceId"`
	Name        string `json:"Name"`
	Description string `json:"Description,omitempty"`
	// Spec holds the pipeline definition YAML.
	Spec       string              `json:"Spec"`
	Parameters []PipelineParameter `json:"Parameters,omitempty"`
}

// RunPipelineParam overrides a dynamic parameter for a single run. The exact CP
// field name (Params vs Variables) is unconfirmed and may need a one-line change
// when integrating against the live API.
type RunPipelineParam struct {
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

// RunPipelineInput triggers a pipeline run with per-run parameter overrides.
// CP identifies the pipeline via "Id" (not "PipelineId") on this action.
type RunPipelineInput struct {
	WorkspaceID string             `json:"WorkspaceId"`
	PipelineID  string             `json:"Id"`
	Params      []RunPipelineParam `json:"Params,omitempty"`
}

// GetTaskRunLogInput identifies a single step's log within a task run.
type GetTaskRunLogInput struct {
	WorkspaceID   string
	PipelineID    string
	PipelineRunID string
	TaskRunID     string
	TaskID        string
	StepName      string
	Offset        int
	Limit         int
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
	DeletePipeline(ctx context.Context, workspaceID, id string) error

	RunPipeline(ctx context.Context, in RunPipelineInput) (PipelineRun, error)
	GetPipelineRun(ctx context.Context, workspaceID, pipelineID, runID string) (PipelineRun, error)
	CancelPipelineRun(ctx context.Context, workspaceID, pipelineID, runID string) error

	ListTaskRuns(ctx context.Context, workspaceID, pipelineID, runID, taskID string) ([]TaskRun, error)
	GetTaskRunLog(ctx context.Context, in GetTaskRunLogInput) (LogPage, error)
}
