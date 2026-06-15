package api

import (
	"encoding/json"
	"net/http"

	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/config"
)

type Router struct {
	mux *http.ServeMux
	cfg config.Config
}

type errorEnvelope struct {
	Error apiError `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewRouter(cfg config.Config) http.Handler {
	r := &Router{mux: http.NewServeMux(), cfg: cfg}
	r.routes()
	return r
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.mux.ServeHTTP(w, req)
}

func (r *Router) routes() {
	r.mux.HandleFunc("/healthz", r.health)
	r.mux.HandleFunc("/api/config", r.config)
	r.mux.HandleFunc("/", r.notFound)
}

func (r *Router) health(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (r *Router) config(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, PublicConfigFrom(r.cfg))
}

func (r *Router) notFound(w http.ResponseWriter, req *http.Request) {
	writeError(w, http.StatusNotFound, "not_found", "route not found")
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorEnvelope{Error: apiError{Code: code, Message: message}})
}

type PublicConfig struct {
	HTTPAddr          string                    `json:"httpAddr"`
	VolcTarget        string                    `json:"volcTarget"`
	TOS               config.TOSConfig          `json:"tos"`
	Dataset           config.DatasetConfig      `json:"dataset"`
	Materializer      config.MaterializerConfig `json:"materializer"`
	RegistryNamespace string                    `json:"registryNamespace"`
	Concurrency       config.ConcurrencyConfig  `json:"concurrency"`
	CP                config.CPConfig           `json:"cp"`
	MockMode          bool                      `json:"mockMode"`
	Secrets           SecretPresence            `json:"secrets"`
}

type SecretPresence struct {
	VolcAccessKey bool `json:"volcAccessKey"`
	VolcSecretKey bool `json:"volcSecretKey"`
	DatabaseURL   bool `json:"databaseUrl"`
}

func PublicConfigFrom(cfg config.Config) PublicConfig {
	return PublicConfig{
		HTTPAddr:          cfg.HTTPAddr,
		VolcTarget:        cfg.VolcTarget,
		TOS:               cfg.TOS,
		Dataset:           cfg.Dataset,
		Materializer:      cfg.Materializer,
		RegistryNamespace: cfg.RegistryNamespace,
		Concurrency:       cfg.Concurrency,
		CP:                cfg.CP,
		MockMode:          cfg.MockMode,
		Secrets: SecretPresence{
			DatabaseURL: cfg.DatabaseURL != "",
		},
	}
}
