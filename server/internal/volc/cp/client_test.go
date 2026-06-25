package cp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func newTestHTTPClient(t *testing.T, handler http.HandlerFunc) *HTTPClient {
	t.Helper()
	srv := httptest.NewTLSServer(handler)
	t.Cleanup(srv.Close)

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	c := NewHTTPClient(Target{Host: u.Host, Region: "cn-beijing", Service: "cp", Version: "2023-01-01"},
		Credentials{AccessKey: "AK", SecretKey: "SK"})
	c.http = srv.Client()
	c.now = func() time.Time { return time.Date(2024, 6, 19, 7, 13, 6, 0, time.UTC) }
	return c
}

func TestHTTPClientDecodesResult(t *testing.T) {
	c := newTestHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			t.Errorf("request not signed")
		}
		if r.URL.Query().Get("Action") != "CreateWorkspace" {
			t.Errorf("action = %s", r.URL.Query().Get("Action"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ResponseMetadata":{"RequestId":"req-1"},"Result":{"Id":"ws-9","Name":"base"}}`))
	})

	ws, err := c.CreateWorkspace(context.Background(), CreateWorkspaceInput{Name: "base"})
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	if ws.ID != "ws-9" || ws.Name != "base" {
		t.Fatalf("workspace = %#v", ws)
	}
}

func TestHTTPClientMapsAPIError(t *testing.T) {
	c := newTestHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ResponseMetadata":{"RequestId":"req-2","Error":{"Code":"InvalidParameter","Message":"bad name"}}}`))
	})

	_, err := c.CreateWorkspace(context.Background(), CreateWorkspaceInput{Name: ""})
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("err = %v, want APIError", err)
	}
	if apiErr.Code != "InvalidParameter" || apiErr.RequestID != "req-2" || apiErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("apiErr = %#v", apiErr)
	}
	if !strings.Contains(apiErr.Error(), "bad name") {
		t.Fatalf("error string = %s", apiErr.Error())
	}
}

func TestHTTPClientErrorEnvelopeOn200(t *testing.T) {
	c := newTestHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ResponseMetadata":{"RequestId":"req-3","Error":{"Code":"QuotaExceeded","Message":"too many"}}}`))
	})

	_, err := c.RunPipeline(context.Background(), RunPipelineInput{PipelineID: "p"})
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Code != "QuotaExceeded" {
		t.Fatalf("err = %v", err)
	}
}
