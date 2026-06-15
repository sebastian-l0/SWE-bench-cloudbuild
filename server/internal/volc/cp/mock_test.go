package cp

import (
	"context"
	"errors"
	"testing"
)

func TestMockClientLifecycle(t *testing.T) {
	ctx := context.Background()
	m := NewMockClient()

	ws, err := m.CreateWorkspace(ctx, CreateWorkspaceInput{Name: "base"})
	if err != nil || ws.ID == "" {
		t.Fatalf("CreateWorkspace = %#v, %v", ws, err)
	}

	sc, err := m.CreateServiceConnection(ctx, CreateServiceConnectionInput{Name: "registry", Type: "registry"})
	if err != nil || sc.ID == "" {
		t.Fatalf("CreateServiceConnection = %#v, %v", sc, err)
	}
	if got, err := m.GetServiceConnection(ctx, sc.ID); err != nil || got.ID != sc.ID {
		t.Fatalf("GetServiceConnection = %#v, %v", got, err)
	}

	p, err := m.CreatePipeline(ctx, CreatePipelineInput{WorkspaceID: ws.ID, Name: "build"})
	if err != nil || p.ID == "" {
		t.Fatalf("CreatePipeline = %#v, %v", p, err)
	}
	pls, err := m.ListPipelines(ctx, ws.ID)
	if err != nil || len(pls) != 1 {
		t.Fatalf("ListPipelines = %v, %v", pls, err)
	}

	run, err := m.RunPipeline(ctx, RunPipelineInput{PipelineID: p.ID, Params: []RunPipelineParam{{Key: "tag", Value: "x"}}})
	if err != nil || run.Status != "Running" {
		t.Fatalf("RunPipeline = %#v, %v", run, err)
	}

	// First read transitions the run to Succeeded.
	got, err := m.GetPipelineRun(ctx, run.ID)
	if err != nil || got.Status != "Succeeded" {
		t.Fatalf("GetPipelineRun = %#v, %v", got, err)
	}
	tasks, err := m.ListTaskRuns(ctx, run.ID)
	if err != nil || len(tasks) != 1 || tasks[0].Status != "Succeeded" {
		t.Fatalf("ListTaskRuns = %v, %v", tasks, err)
	}

	logPage, err := m.GetTaskRunLog(ctx, tasks[0].ID, "")
	if err != nil || logPage.Content == "" {
		t.Fatalf("GetTaskRunLog = %#v, %v", logPage, err)
	}
}

func TestMockClientUnknownIDErrors(t *testing.T) {
	ctx := context.Background()
	m := NewMockClient()

	var apiErr *APIError
	err := m.DeleteWorkspace(ctx, "missing")
	if !errors.As(err, &apiErr) || apiErr.HTTPStatus != 404 {
		t.Fatalf("DeleteWorkspace unknown err = %v", err)
	}

	if _, err := m.RunPipeline(ctx, RunPipelineInput{PipelineID: "missing"}); !errors.As(err, &apiErr) {
		t.Fatalf("RunPipeline unknown pipeline err = %v", err)
	}
	if _, err := m.GetPipelineRun(ctx, "missing"); !errors.As(err, &apiErr) {
		t.Fatalf("GetPipelineRun unknown err = %v", err)
	}
}

func TestMockClientCancelRun(t *testing.T) {
	ctx := context.Background()
	m := NewMockClient()
	ws, _ := m.CreateWorkspace(ctx, CreateWorkspaceInput{Name: "w"})
	p, _ := m.CreatePipeline(ctx, CreatePipelineInput{WorkspaceID: ws.ID, Name: "p"})
	run, _ := m.RunPipeline(ctx, RunPipelineInput{PipelineID: p.ID})

	if err := m.CancelPipelineRun(ctx, run.ID); err != nil {
		t.Fatalf("CancelPipelineRun: %v", err)
	}
	got, _ := m.GetPipelineRun(ctx, run.ID)
	if got.Status != "Canceled" {
		t.Fatalf("status after cancel = %q", got.Status)
	}
}
