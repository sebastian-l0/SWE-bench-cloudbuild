package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/sebastian-l0/SWE-bench-cloudbuild/server/internal/model"
)

// PostgresStore is a PostgreSQL-backed Store implementation using pgx.
type PostgresStore struct {
	pool *pgxpool.Pool
}

var _ Store = (*PostgresStore)(nil)

// NewPostgresStore connects to PostgreSQL using databaseURL and verifies the
// connection. The caller owns the returned store and must call Close.
func NewPostgresStore(ctx context.Context, databaseURL string) (*PostgresStore, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("store: connect: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping: %w", err)
	}
	return &PostgresStore{pool: pool}, nil
}

// Close releases the underlying connection pool.
func (s *PostgresStore) Close() {
	s.pool.Close()
}

// ApplyMigrations executes every *.sql file in migrationsDir in lexical order.
// Migrations are expected to be idempotent (CREATE TABLE IF NOT EXISTS ...).
func (s *PostgresStore) ApplyMigrations(ctx context.Context, migrationsDir string) error {
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("store: read migrations: %w", err)
	}
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".sql" {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)
	for _, name := range files {
		sqlBytes, err := os.ReadFile(filepath.Join(migrationsDir, name))
		if err != nil {
			return fmt.Errorf("store: read migration %s: %w", name, err)
		}
		if _, err := s.pool.Exec(ctx, string(sqlBytes)); err != nil {
			return fmt.Errorf("store: apply migration %s: %w", name, err)
		}
	}
	return nil
}

func (s *PostgresStore) CreateRun(ctx context.Context, run model.Run) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO runs (id, name, status, phase, dataset, output_dir, tos_bucket, tos_prefix, registry, manifest_json, error, created_at, started_at, finished_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		run.ID, run.Name, run.Status, run.Phase, run.Dataset, run.OutputDir, run.TOSBucket, run.TOSPrefix, run.Registry,
		jsonbOrNil(run.ManifestJSON), run.Error, run.CreatedAt, run.StartedAt, run.FinishedAt)
	return wrapExec(err)
}

func (s *PostgresStore) GetRun(ctx context.Context, id string) (model.Run, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, name, status, phase, dataset, output_dir, tos_bucket, tos_prefix, registry,
		       COALESCE(manifest_json::text, ''), error, created_at, started_at, finished_at
		FROM runs WHERE id = $1`, id)
	return scanRun(row)
}

func (s *PostgresStore) ListRuns(ctx context.Context) ([]model.Run, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, name, status, phase, dataset, output_dir, tos_bucket, tos_prefix, registry,
		       COALESCE(manifest_json::text, ''), error, created_at, started_at, finished_at
		FROM runs ORDER BY created_at DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.Run, 0)
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, run)
	}
	return out, rows.Err()
}

func (s *PostgresStore) UpdateRun(ctx context.Context, run model.Run) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE runs SET name=$2, status=$3, phase=$4, dataset=$5, output_dir=$6, tos_bucket=$7, tos_prefix=$8,
		       registry=$9, manifest_json=$10, error=$11, started_at=$12, finished_at=$13
		WHERE id=$1`,
		run.ID, run.Name, run.Status, run.Phase, run.Dataset, run.OutputDir, run.TOSBucket, run.TOSPrefix, run.Registry,
		jsonbOrNil(run.ManifestJSON), run.Error, run.StartedAt, run.FinishedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) CreateImageBuild(ctx context.Context, img model.ImageBuild) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO image_builds (id, run_id, layer, local_key, target_image, context_path, dockerfile,
		       depends_on_key, workspace_id, pipeline_id, status, last_run_id, attempts, error, raw_manifest, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`,
		img.ID, img.RunID, img.Layer, img.LocalKey, img.TargetImage, img.ContextPath, img.Dockerfile,
		img.DependsOnKey, img.WorkspaceID, img.PipelineID, img.Status, img.LastRunID, img.Attempts, img.Error,
		jsonbOrNil(img.RawManifest), img.CreatedAt, img.UpdatedAt)
	return wrapExec(err)
}

func (s *PostgresStore) GetImageBuild(ctx context.Context, id string) (model.ImageBuild, error) {
	row := s.pool.QueryRow(ctx, `
		SELECT id, run_id, layer, local_key, target_image, context_path, dockerfile, depends_on_key,
		       workspace_id, pipeline_id, status, last_run_id, attempts, error,
		       COALESCE(raw_manifest::text, ''), created_at, updated_at
		FROM image_builds WHERE id=$1`, id)
	return scanImageBuild(row)
}

func (s *PostgresStore) ListImageBuildsByRun(ctx context.Context, runID string) ([]model.ImageBuild, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, run_id, layer, local_key, target_image, context_path, dockerfile, depends_on_key,
		       workspace_id, pipeline_id, status, last_run_id, attempts, error,
		       COALESCE(raw_manifest::text, ''), created_at, updated_at
		FROM image_builds WHERE run_id=$1 ORDER BY created_at, id`, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.ImageBuild, 0)
	for rows.Next() {
		img, err := scanImageBuild(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, img)
	}
	return out, rows.Err()
}

func (s *PostgresStore) UpdateImageBuild(ctx context.Context, img model.ImageBuild) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE image_builds SET layer=$2, local_key=$3, target_image=$4, context_path=$5, dockerfile=$6,
		       depends_on_key=$7, workspace_id=$8, pipeline_id=$9, status=$10, last_run_id=$11,
		       attempts=$12, error=$13, raw_manifest=$14, updated_at=$15
		WHERE id=$1`,
		img.ID, img.Layer, img.LocalKey, img.TargetImage, img.ContextPath, img.Dockerfile,
		img.DependsOnKey, img.WorkspaceID, img.PipelineID, img.Status, img.LastRunID, img.Attempts, img.Error,
		jsonbOrNil(img.RawManifest), img.UpdatedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) CreateAttempt(ctx context.Context, attempt model.RunAttempt) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO run_attempts (id, image_build_id, pipeline_run_id, status, log_url, started_at, updated_at, finished_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		attempt.ID, attempt.ImageBuildID, attempt.PipelineRunID, attempt.Status, attempt.LogURL,
		attempt.StartedAt, attempt.UpdatedAt, attempt.FinishedAt)
	return wrapExec(err)
}

func (s *PostgresStore) UpdateAttempt(ctx context.Context, attempt model.RunAttempt) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE run_attempts SET pipeline_run_id=$2, status=$3, log_url=$4, updated_at=$5, finished_at=$6
		WHERE id=$1`,
		attempt.ID, attempt.PipelineRunID, attempt.Status, attempt.LogURL, attempt.UpdatedAt, attempt.FinishedAt)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) ListAttemptsByImage(ctx context.Context, imageBuildID string) ([]model.RunAttempt, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, image_build_id, pipeline_run_id, status, log_url, started_at, updated_at, finished_at
		FROM run_attempts WHERE image_build_id=$1 ORDER BY started_at, id`, imageBuildID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.RunAttempt, 0)
	for rows.Next() {
		var a model.RunAttempt
		if err := rows.Scan(&a.ID, &a.ImageBuildID, &a.PipelineRunID, &a.Status, &a.LogURL,
			&a.StartedAt, &a.UpdatedAt, &a.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *PostgresStore) AppendEvent(ctx context.Context, event model.RunEvent) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO run_events (id, run_id, image_id, type, payload, created_at)
		VALUES ($1,$2,$3,$4,$5,$6)`,
		event.ID, event.RunID, event.ImageID, event.Type, jsonbOrDefault(event.Payload), event.CreatedAt)
	return wrapExec(err)
}

func (s *PostgresStore) ListEventsByRun(ctx context.Context, runID string, afterID string) ([]model.RunEvent, error) {
	const query = `
		SELECT id, run_id, image_id, type, COALESCE(payload::text, ''), created_at
		FROM run_events
		WHERE run_id=$1
		  AND ($2 = '' OR created_at > (SELECT created_at FROM run_events WHERE id=$2))
		ORDER BY created_at, id`
	rows, err := s.pool.Query(ctx, query, runID, afterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]model.RunEvent, 0)
	for rows.Next() {
		var e model.RunEvent
		if err := rows.Scan(&e.ID, &e.RunID, &e.ImageID, &e.Type, &e.Payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// rowScanner abstracts pgx.Row and pgx.Rows for shared scan helpers.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanRun(row rowScanner) (model.Run, error) {
	var r model.Run
	err := row.Scan(&r.ID, &r.Name, &r.Status, &r.Phase, &r.Dataset, &r.OutputDir, &r.TOSBucket, &r.TOSPrefix, &r.Registry,
		&r.ManifestJSON, &r.Error, &r.CreatedAt, &r.StartedAt, &r.FinishedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Run{}, ErrNotFound
	}
	return r, err
}

func scanImageBuild(row rowScanner) (model.ImageBuild, error) {
	var img model.ImageBuild
	err := row.Scan(&img.ID, &img.RunID, &img.Layer, &img.LocalKey, &img.TargetImage, &img.ContextPath,
		&img.Dockerfile, &img.DependsOnKey, &img.WorkspaceID, &img.PipelineID, &img.Status, &img.LastRunID,
		&img.Attempts, &img.Error, &img.RawManifest, &img.CreatedAt, &img.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.ImageBuild{}, ErrNotFound
	}
	return img, err
}

// jsonbOrNil returns nil for empty strings so JSONB columns stay NULL instead of
// failing to parse an empty document.
func jsonbOrNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// jsonbOrDefault returns an empty JSON object for empty payloads.
func jsonbOrDefault(s string) any {
	if s == "" {
		return "{}"
	}
	return s
}

// wrapExec maps PostgreSQL unique-violation errors to ErrDuplicate.
func wrapExec(err error) error {
	if err == nil {
		return nil
	}
	if isUniqueViolation(err) {
		return ErrDuplicate
	}
	return err
}
