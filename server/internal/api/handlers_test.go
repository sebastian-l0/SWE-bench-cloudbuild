package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/config"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/model"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/service"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/store"
)

func newTestService(t *testing.T) (*service.Service, store.Store) {
	t.Helper()
	cfg := config.Defaults()
	cfg.MockMode = true
	st := store.NewMemoryStore()
	svc, err := service.New(service.Options{Config: cfg, Store: st})
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	return svc, st
}

func TestCreateAndRunEndToEndMock(t *testing.T) {
	svc, st := newTestService(t)
	h := NewRouterWithService(svc)

	// Create a run using the generated-directory fixture.
	body, _ := json.Marshal(map[string]string{
		"name":      "demo",
		"outputDir": "../manifest/testdata/valid",
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/runs", bytes.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d body=%s", rec.Code, rec.Body.String())
	}
	var run model.Run
	if err := json.Unmarshal(rec.Body.Bytes(), &run); err != nil {
		t.Fatalf("decode run: %v", err)
	}
	if run.ID == "" {
		t.Fatal("run id empty")
	}

	// Start the run.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/runs/"+run.ID+"/start", nil))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("start status = %d body=%s", rec.Code, rec.Body.String())
	}

	// Poll the run detail until terminal.
	deadline := time.Now().Add(5 * time.Second)
	var finalStatus string
	for time.Now().Before(deadline) {
		r, _ := st.GetRun(context.Background(), run.ID)
		finalStatus = r.Status
		if finalStatus == "success" || finalStatus == "failed" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if finalStatus != "success" {
		t.Fatalf("final run status = %q, want success", finalStatus)
	}

	// Run detail returns images and summary.
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/runs/"+run.ID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("detail status = %d", rec.Code)
	}
	var detail struct {
		Run     model.Run                 `json:"run"`
		Images  []model.ImageBuild        `json:"images"`
		Summary map[string]map[string]int `json:"summary"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if len(detail.Images) != 3 {
		t.Fatalf("images = %d, want 3", len(detail.Images))
	}

	// Image log returns content.
	imgID := detail.Images[0].ID
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/images/"+imgID+"/log", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("log status = %d", rec.Code)
	}
	var logResp struct {
		Log string `json:"log"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &logResp)
	if logResp.Log == "" {
		t.Fatal("expected log content")
	}
}

func TestRunNotFound(t *testing.T) {
	svc, _ := newTestService(t)
	h := NewRouterWithService(svc)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/runs/missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestListRunsEmpty(t *testing.T) {
	svc, _ := newTestService(t)
	h := NewRouterWithService(svc)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/runs", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp struct {
		Runs []model.Run `json:"runs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Runs) != 0 {
		t.Fatalf("runs = %d, want 0", len(resp.Runs))
	}
}

func TestPutConfigAppliesOverridesWithoutLeak(t *testing.T) {
	svc, _ := newTestService(t)
	h := NewRouterWithService(svc)

	secret := "AKIA" + "putconfig"
	body, _ := json.Marshal(map[string]any{
		"registryNamespace": "reg.example.com/swe",
		"volcAccessKey":     secret,
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("put status = %d body=%s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte(secret)) {
		t.Fatalf("response leaked secret: %s", rec.Body.String())
	}

	var pub PublicConfig
	if err := json.Unmarshal(rec.Body.Bytes(), &pub); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if pub.RegistryNamespace != "reg.example.com/swe" {
		t.Fatalf("registry = %q", pub.RegistryNamespace)
	}
	if !pub.Secrets.VolcAccessKey {
		t.Fatalf("expected volc access key presence true")
	}
}
