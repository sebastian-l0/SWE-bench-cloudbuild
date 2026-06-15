package main

import (
	"log"
	"net/http"

	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/api"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/config"
)

func main() {
	cfg := config.Load()
	handler := api.NewRouter(cfg)

	log.Printf("starting swe-cloudbuild server on %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, handler); err != nil {
		log.Fatal(err)
	}
}
