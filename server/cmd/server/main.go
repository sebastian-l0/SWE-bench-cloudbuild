package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/api"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/config"
	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/store"
)

func main() {
	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pg, err := store.NewPostgresStore(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}
	defer pg.Close()
	if err := pg.ApplyMigrations(ctx, "migrations"); err != nil {
		log.Fatalf("apply migrations: %v", err)
	}

	handler := api.NewRouter(cfg)

	log.Printf("starting swe-cloudbuild server on %s", cfg.HTTPAddr)
	if err := http.ListenAndServe(cfg.HTTPAddr, handler); err != nil {
		log.Fatal(err)
	}
}
