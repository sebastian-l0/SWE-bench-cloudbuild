CREATE TABLE IF NOT EXISTS runs (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    status TEXT NOT NULL,
    phase TEXT NOT NULL,
    dataset TEXT NOT NULL,
    tos_bucket TEXT NOT NULL,
    tos_prefix TEXT NOT NULL,
    registry TEXT NOT NULL,
    manifest_json JSONB,
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS image_builds (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    layer TEXT NOT NULL,
    local_key TEXT NOT NULL,
    target_image TEXT NOT NULL,
    context_path TEXT NOT NULL,
    dockerfile TEXT NOT NULL,
    depends_on_key TEXT NOT NULL DEFAULT '',
    workspace_id TEXT NOT NULL DEFAULT '',
    pipeline_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    last_run_id TEXT NOT NULL DEFAULT '',
    attempts INTEGER NOT NULL DEFAULT 0,
    error TEXT NOT NULL DEFAULT '',
    raw_manifest JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(run_id, local_key)
);

CREATE TABLE IF NOT EXISTS run_attempts (
    id TEXT PRIMARY KEY,
    image_build_id TEXT NOT NULL REFERENCES image_builds(id) ON DELETE CASCADE,
    pipeline_run_id TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    log_url TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS run_events (
    id TEXT PRIMARY KEY,
    run_id TEXT NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    image_id TEXT NOT NULL DEFAULT '',
    type TEXT NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_image_builds_run_layer ON image_builds(run_id, layer);
CREATE INDEX IF NOT EXISTS idx_run_attempts_image ON run_attempts(image_build_id);
CREATE INDEX IF NOT EXISTS idx_run_events_run_created ON run_events(run_id, created_at);
