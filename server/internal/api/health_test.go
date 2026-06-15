package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/config"
)

func TestHealthEndpoint(t *testing.T) {
	h := NewRouter(config.Defaults())
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "{\"status\":\"ok\"}\n" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestNotFoundUsesJSONErrorEnvelope(t *testing.T) {
	h := NewRouter(config.Defaults())
	req := httptest.NewRequest(http.MethodGet, "/missing", nil)
	rec := httptest.NewRecorder()

	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "{\"error\":{\"code\":\"not_found\",\"message\":\"route not found\"}}\n" {
		t.Fatalf("body = %q", rec.Body.String())
	}
}
