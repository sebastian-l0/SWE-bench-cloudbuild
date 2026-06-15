package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestPublicConfigExposesSecretPresenceWithoutValues(t *testing.T) {
	cfg := config.Defaults()
	cfg.VolcAccessKey = "AKIA" + "publictest"
	cfg.VolcSecretKey = "secret" + "publictest"

	pub := PublicConfigFrom(cfg)

	if !pub.Secrets.VolcAccessKey || !pub.Secrets.VolcSecretKey || !pub.Secrets.DatabaseURL {
		t.Fatalf("secret presence = %#v, want all true", pub.Secrets)
	}

	body, err := json.Marshal(pub)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, leaked := range []string{cfg.VolcAccessKey, cfg.VolcSecretKey, cfg.DatabaseURL} {
		if strings.Contains(string(body), leaked) {
			t.Fatalf("public config leaked secret %q in %s", leaked, body)
		}
	}
}
