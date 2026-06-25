package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/model"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/service"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/store"
)

// configUpdateBody is the PUT /api/config request. Pointer fields distinguish
// "omitted" from "set to empty"; only provided fields are applied.
type configUpdateBody struct {
	VolcTarget        *string `json:"volcTarget"`
	VolcAccessKey     *string `json:"volcAccessKey"`
	VolcSecretKey     *string `json:"volcSecretKey"`
	TOSBucket         *string `json:"tosBucket"`
	TOSParentPath     *string `json:"tosParentPath"`
	RegistryNamespace *string `json:"registryNamespace"`
	MockMode          *bool   `json:"mockMode"`
}

func (b configUpdateBody) toServiceUpdate() service.ConfigUpdate {
	return service.ConfigUpdate{
		VolcTarget:        b.VolcTarget,
		VolcAccessKey:     b.VolcAccessKey,
		VolcSecretKey:     b.VolcSecretKey,
		TOSBucket:         b.TOSBucket,
		TOSParentPath:     b.TOSParentPath,
		RegistryNamespace: b.RegistryNamespace,
		MockMode:          b.MockMode,
	}
}

// runsCollection handles GET /api/runs and POST /api/runs.
func (r *Router) runsCollection(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		runs, err := r.svc.ListRuns(req.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
	case http.MethodPost:
		var body struct {
			Name      string `json:"name"`
			OutputDir string `json:"outputDir"`
			Dataset   string `json:"dataset"`
		}
		if req.Body != nil {
			_ = json.NewDecoder(req.Body).Decode(&body)
		}
		run, err := r.svc.CreateRun(req.Context(), service.CreateRunInput{
			Name: body.Name, OutputDir: body.OutputDir, Dataset: body.Dataset,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, run)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

// runsItem handles /api/runs/:id and subresources.
func (r *Router) runsItem(w http.ResponseWriter, req *http.Request) {
	rest := strings.TrimPrefix(req.URL.Path, "/api/runs/")
	parts := strings.Split(rest, "/")
	id := parts[0]
	if id == "" {
		writeError(w, http.StatusNotFound, "not_found", "run id required")
		return
	}

	// /api/runs/:id/<action>
	if len(parts) >= 2 {
		switch parts[1] {
		case "start":
			r.requirePost(w, req, func() {
				if err := r.svc.StartRun(id); err != nil {
					writeError(w, http.StatusConflict, "conflict", err.Error())
					return
				}
				writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
			})
		case "cancel":
			r.requirePost(w, req, func() {
				if err := r.svc.CancelRun(req.Context(), id); err != nil {
					r.writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusOK, map[string]string{"status": "canceled"})
			})
		case "events":
			r.streamEvents(w, req, id)
		default:
			writeError(w, http.StatusNotFound, "not_found", "unknown run action")
		}
		return
	}

	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	run, images, err := r.svc.GetRun(req.Context(), id)
	if err != nil {
		r.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": run, "images": images, "summary": layerSummary(images)})
}

// imagesItem handles /api/images/:id, /api/images/:id/retry, /api/images/:id/log.
func (r *Router) imagesItem(w http.ResponseWriter, req *http.Request) {
	rest := strings.TrimPrefix(req.URL.Path, "/api/images/")
	parts := strings.Split(rest, "/")
	id := parts[0]
	if id == "" {
		writeError(w, http.StatusNotFound, "not_found", "image id required")
		return
	}

	if len(parts) >= 2 {
		switch parts[1] {
		case "retry":
			r.requirePost(w, req, func() {
				if err := r.svc.RetryImage(req.Context(), id); err != nil {
					r.writeStoreError(w, err)
					return
				}
				writeJSON(w, http.StatusAccepted, map[string]string{"status": "retried"})
			})
		case "log":
			if req.Method != http.MethodGet {
				writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
				return
			}
			logContent, err := r.svc.ImageLog(req.Context(), id)
			if err != nil {
				r.writeStoreError(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"log": logContent})
		default:
			writeError(w, http.StatusNotFound, "not_found", "unknown image action")
		}
		return
	}

	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	img, err := r.svc.GetImage(req.Context(), id)
	if err != nil {
		r.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, img)
}

func (r *Router) requirePost(w http.ResponseWriter, req *http.Request, fn func()) {
	if req.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	fn()
}

func (r *Router) writeStoreError(w http.ResponseWriter, err error) {
	if err == store.ErrNotFound {
		writeError(w, http.StatusNotFound, "not_found", "resource not found")
		return
	}
	writeError(w, http.StatusInternalServerError, "internal", err.Error())
}

// streamEvents serves Server-Sent Events for a run, resuming from Last-Event-ID.
func (r *Router) streamEvents(w http.ResponseWriter, req *http.Request, runID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal", "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	lastID := req.Header.Get("Last-Event-ID")
	ctx := req.Context()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	send := func() bool {
		events, err := r.svc.Events(ctx, runID, lastID)
		if err != nil {
			return true
		}
		for _, ev := range events {
			writeSSE(w, ev)
			lastID = ev.ID
		}
		flusher.Flush()
		return false
	}

	// Initial flush of backlog.
	if stop := send(); stop {
		return
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if stop := send(); stop {
				return
			}
		}
	}
}

func writeSSE(w http.ResponseWriter, ev model.RunEvent) {
	fmt.Fprintf(w, "id: %s\n", ev.ID)
	fmt.Fprintf(w, "event: %s\n", ev.Type)
	fmt.Fprintf(w, "data: %s\n\n", ev.Payload)
}

// layerSummary tallies image statuses per layer for the run detail response.
func layerSummary(images []model.ImageBuild) map[string]map[string]int {
	out := map[string]map[string]int{}
	for _, img := range images {
		if out[img.Layer] == nil {
			out[img.Layer] = map[string]int{}
		}
		out[img.Layer][img.Status]++
		out[img.Layer]["total"]++
	}
	return out
}
