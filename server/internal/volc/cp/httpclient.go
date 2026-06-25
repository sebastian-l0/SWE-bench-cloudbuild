package cp

import (
	"context"
	"net/http"
	"strconv"
)

var _ Client = (*HTTPClient)(nil)

func (c *HTTPClient) CreateWorkspace(ctx context.Context, in CreateWorkspaceInput) (Workspace, error) {
	var out Workspace
	err := c.Call(ctx, http.MethodPost, "CreateWorkspace", in, &out)
	return out, err
}

func (c *HTTPClient) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	var out struct {
		Items []Workspace `json:"Items"`
	}
	err := c.Call(ctx, http.MethodGet, "ListWorkspaces", nil, &out)
	return out.Items, err
}

func (c *HTTPClient) DeleteWorkspace(ctx context.Context, id string) error {
	return c.Call(ctx, http.MethodPost, "DeleteWorkspace", map[string]string{"Id": id}, nil)
}

func (c *HTTPClient) CreateServiceConnection(ctx context.Context, in CreateServiceConnectionInput) (ServiceConnection, error) {
	var out ServiceConnection
	err := c.Call(ctx, http.MethodPost, "CreateServiceConnection", in, &out)
	return out, err
}

func (c *HTTPClient) GetServiceConnection(ctx context.Context, id string) (ServiceConnection, error) {
	var out ServiceConnection
	err := c.Call(ctx, http.MethodGet, "GetServiceConnection", map[string]string{"Id": id}, &out)
	return out, err
}

func (c *HTTPClient) ListServiceConnections(ctx context.Context) ([]ServiceConnection, error) {
	var out struct {
		Items []ServiceConnection `json:"Items"`
	}
	err := c.Call(ctx, http.MethodGet, "ListServiceConnections", nil, &out)
	return out.Items, err
}

func (c *HTTPClient) DeleteServiceConnection(ctx context.Context, id string) error {
	return c.Call(ctx, http.MethodPost, "DeleteServiceConnection", map[string]string{"Id": id}, nil)
}

func (c *HTTPClient) CreatePipeline(ctx context.Context, in CreatePipelineInput) (Pipeline, error) {
	var out Pipeline
	err := c.Call(ctx, http.MethodPost, "CreatePipeline", in, &out)
	return out, err
}

func (c *HTTPClient) ListPipelines(ctx context.Context, workspaceID string) ([]Pipeline, error) {
	var out struct {
		Items []Pipeline `json:"Items"`
	}
	err := c.Call(ctx, http.MethodGet, "ListPipelines", map[string]string{"WorkspaceId": workspaceID}, &out)
	return out.Items, err
}

func (c *HTTPClient) DeletePipeline(ctx context.Context, workspaceID, id string) error {
	return c.Call(ctx, http.MethodPost, "DeletePipeline", map[string]string{"WorkspaceId": workspaceID, "Id": id}, nil)
}

func (c *HTTPClient) RunPipeline(ctx context.Context, in RunPipelineInput) (PipelineRun, error) {
	var out PipelineRun
	err := c.Call(ctx, http.MethodPost, "RunPipeline", in, &out)
	return out, err
}

// GetPipelineRun fetches a run via ListPipelineRuns and selects the matching id.
// CP has no single-run GET action on this version; ListPipelineRuns returns runs
// with nested Stages/Tasks.
func (c *HTTPClient) GetPipelineRun(ctx context.Context, workspaceID, pipelineID, runID string) (PipelineRun, error) {
	var out struct {
		Items []PipelineRun `json:"Items"`
	}
	err := c.Call(ctx, http.MethodGet, "ListPipelineRuns",
		map[string]string{"WorkspaceId": workspaceID, "PipelineId": pipelineID}, &out)
	if err != nil {
		return PipelineRun{}, err
	}
	for _, run := range out.Items {
		if run.ID == runID {
			return run, nil
		}
	}
	return PipelineRun{}, &APIError{HTTPStatus: 404, Code: "NotFound",
		Message: "pipeline run " + runID + " not found"}
}

func (c *HTTPClient) CancelPipelineRun(ctx context.Context, workspaceID, pipelineID, runID string) error {
	return c.Call(ctx, http.MethodPost, "CancelPipelineRun",
		map[string]string{"WorkspaceId": workspaceID, "PipelineId": pipelineID, "Id": runID}, nil)
}

func (c *HTTPClient) ListTaskRuns(ctx context.Context, workspaceID, pipelineID, runID, taskID string) ([]TaskRun, error) {
	var out struct {
		Items []TaskRun `json:"Items"`
	}
	err := c.Call(ctx, http.MethodGet, "ListTaskRuns", map[string]string{
		"WorkspaceId":   workspaceID,
		"PipelineId":    pipelineID,
		"PipelineRunId": runID,
		"TaskId":        taskID,
	}, &out)
	return out.Items, err
}

func (c *HTTPClient) GetTaskRunLog(ctx context.Context, in GetTaskRunLogInput) (LogPage, error) {
	var out LogPage
	params := map[string]string{
		"WorkspaceId":   in.WorkspaceID,
		"PipelineId":    in.PipelineID,
		"PipelineRunId": in.PipelineRunID,
		"TaskRunId":     in.TaskRunID,
		"TaskId":        in.TaskID,
		"StepName":      in.StepName,
	}
	if in.Offset > 0 {
		params["Offset"] = strconv.Itoa(in.Offset)
	}
	if in.Limit > 0 {
		params["Limit"] = strconv.Itoa(in.Limit)
	}
	err := c.Call(ctx, http.MethodGet, "GetTaskRunLog", params, &out)
	return out, err
}
