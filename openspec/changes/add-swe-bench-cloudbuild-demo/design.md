# Design: SWE-bench CloudBuild Demo

## Context

The target product is a locally started web application for building SWE-bench Docker images through Volcengine cloud services. The source document describes three implementation stages:

1. Extend / use a SWE-bench fork so it only generates Dockerfile build contexts for `base`, `env`, and `instance` images.
2. Store generated Dockerfiles in GitHub or another artifact store. For the demo, generated contexts are uploaded to TOS under a configured parent path plus timestamp.
3. Use Volcengine Continuous Delivery (CP) to build images in dependency order: all `base_images` first, all `env_images` after base succeeds, then all `instance_images` after env succeeds.

The current repository already contains a CP client OpenSpec change. This design layers the product workflow above that client.

## Goals / Non-Goals

**Goals**

- Provide a local web UI that guides a user through configuration and a single end-to-end build run.
- Clone/run the materialization capability from `https://github.com/sebastian-l0/SWE-bench/tree/feature/materialize-image-contexts` to generate SWE-bench Dockerfile contexts for the first demo.
- Upload all generated contexts to TOS under `{parent_path}/{timestamp}/`.
- Parse `manifest.json` and model three image layers: `base_images`, `env_images`, `instance_images`.
- Orchestrate CP runs in strict layer order, with configurable per-layer concurrency.
- Show phase-level and image-level progress, logs, failures, retries, and cancellation.
- Support mock mode for demos without real Volcengine/TOS credentials.

**Non-Goals**

- Running SWE-bench evaluation tasks.
- Managing arbitrary Docker build systems outside the generated SWE-bench image contexts.
- Hosted multi-user auth, RBAC, or shared persistent deployment.
- Hosted resource procurement outside CP/TOS/registry scope. The first demo SHALL create CP workspaces and pipelines through CP APIs rather than requiring pre-created CP resources.

## Architecture

```text
web/ React UI
  -> server HTTP API
      -> config store / PostgreSQL
      -> materializer runner
      -> tos uploader
      -> manifest parser
      -> orchestrator / worker pool
      -> volc CP client
      -> SSE event bus
```

The backend is the source of truth. The web UI submits configuration and starts runs, then subscribes to SSE for progress updates. Long-running work is owned by the backend scheduler and persisted in PostgreSQL. Local development uses `arm64v8/postgres:15`, connected through `DATABASE_URL`.

## Components

### Configuration

The UI and API collect:

- Volcengine AK/SK and region/target from both `.env` and UI-entered local configuration. UI-entered values take precedence over `.env`; API responses and logs always return redacted values.
- TOS bucket name and parent path.
- SWE-bench dataset identifier, for example `SWE-bench/SWE-bench` with split `test`.
- Materializer repository/ref, defaulting to `https://github.com/sebastian-l0/SWE-bench` and `feature/materialize-image-contexts`.
- Final image registry namespace, for example `agentkit-platform-2100483201-cn-beijing.cr.volces.com/sebs-io/swebench`.
- CP workspace/pipeline creation settings. The default and first-demo path creates workspaces and parameterized pipelines through CP APIs. Reusing existing resources can remain a later compatibility mode, but is not required for the first demo.
- Per-layer concurrency and layer-gate policy.

Secrets must not be written to logs. Local persistence may store encrypted/obfuscated secret references when available; `.env` is acceptable for the first local demo.

### Materializer Runner

The materializer stage wraps the SWE-bench fork capability. It should support two input modes:

1. Clone or update `https://github.com/sebastian-l0/SWE-bench` at branch `feature/materialize-image-contexts`, then execute its materializer command to generate contexts and `manifest.json`.
2. Use an already generated local directory only for tests and developer diagnostics, not as the primary first-demo path.

The runner records command, repo/ref, dataset, output directory, start/end times, stdout/stderr tail, and final artifact location.

### TOS Upload

The upload stage shells out to `toscli` because the product note explicitly names it. It uploads the generated output directory into:

```text
tos://{bucket}/{parent_path}/{yyyyMMddHHmmss}/
```

The timestamped prefix becomes the immutable remote context root for the run. The backend stores the TOS URI and per-file upload summary. Upload errors mark the run failed before any CP builds are triggered.

### Manifest Parser

The parser reads `manifest.json` from the generated output and normalizes entries into `ImageBuild` records. It preserves original manifest fields and extracts at minimum:

- layer: `base`, `env`, or `instance`
- local key / image name
- target image
- context path
- Dockerfile path
- dependency key (`env -> base`, `instance -> env`) when present

Unknown fields are preserved as JSON for later compatibility.

### CP Orchestrator

The orchestrator executes a run as phases:

1. `materializing_dockerfiles`
2. `uploading_dockerfiles`
3. `building_base_images`
4. `building_env_images`
5. `building_instance_images`
6. terminal: `success`, `failed`, or `canceled`

Layer gates default to strict mode: every image in the current layer must succeed before the next layer starts. A later setting may enable best-effort mode where downstream images whose dependencies succeeded can continue.

Before image builds start, the orchestrator creates the required CP workspaces and parameterized pipelines through the CP client. Each image build then calls `RunPipeline` with variables such as:

- `CONTEXT_ROOT`: timestamped TOS prefix or Git/artifact root
- `CONTEXT_PATH`: manifest context path relative to root
- `DOCKERFILE`: Dockerfile path
- `TARGET_IMAGE`: final registry image
- `BASE_IMAGE` / `ENV_IMAGE`: dependency output image when applicable

Run polling updates image status and emits SSE events. Logs are fetched on demand through the CP client.

### Web UI

The first UI should be intentionally small:

- **Settings / New Run**: configure credentials references, dataset, TOS, registry, and CP pipeline mode.
- **Run List**: show recent runs and terminal status.
- **Run Detail**: show phase timeline, layer cards, progress bars, counts, failed images, start/cancel/retry actions.
- **Image Detail**: show manifest data, CP run IDs, attempts, status history, and logs.

## Data Model

```go
type Run struct {
    ID              string
    Name            string
    Status          string // pending/running/success/failed/canceled
    Phase           string // materializing/uploading/building_base/building_env/building_instance
    Dataset         string
    TosBucket       string
    TosPrefix       string
    Registry        string
    ManifestJSON    string
    Error           string
    CreatedAt       time.Time
    StartedAt       *time.Time
    FinishedAt      *time.Time
}

type ImageBuild struct {
    ID             string
    RunID          string
    Layer          string // base/env/instance
    LocalKey       string
    TargetImage    string
    ContextPath    string
    Dockerfile     string
    DependsOnKey   string
    WorkspaceID    string
    PipelineID     string
    Status         string // pending/queued/running/success/failed/skipped/canceled
    LastRunID      string
    Attempts       int
    Error          string
    RawManifest    string
}

type RunEvent struct {
    ID        string
    RunID     string
    ImageID   string
    Type      string
    Payload   string
    CreatedAt time.Time
}
```

## API Shape

| Method | Path | Purpose |
|---|---|---|
| `GET` | `/api/config` | Read non-secret effective config and defaults |
| `PUT` | `/api/config` | Save local config / secret references |
| `POST` | `/api/runs` | Create a run from config |
| `POST` | `/api/runs/:id/start` | Start materialize -> upload -> build orchestration |
| `POST` | `/api/runs/:id/cancel` | Cancel queued/running work and CP runs when possible |
| `POST` | `/api/runs/:id/retry` | Retry failed images or failed phase if safe |
| `GET` | `/api/runs` | List runs |
| `GET` | `/api/runs/:id` | Run detail with layer summary |
| `GET` | `/api/runs/:id/events` | SSE progress stream |
| `GET` | `/api/images/:id` | Image detail |
| `POST` | `/api/images/:id/retry` | Retry one failed image |
| `GET` | `/api/images/:id/log` | Fetch CP task logs |

## Error Handling

- Materialization failure stops before upload; store command exit code and stderr tail.
- Upload failure stops before CP; store failed file/path if known.
- CP trigger failure marks the image failed and applies layer-gate rules.
- Polling failures use bounded retry with backoff; exhausted polling marks image unknown/failed with actionable error.
- Cancellation attempts to cancel running CP pipeline runs and marks queued work canceled.
- All secrets must be redacted from logs, API responses, and persisted event payloads.

## Testing Strategy

- Unit tests for manifest parsing, dependency graph validation, phase transitions, strict gate behavior, best-effort gate behavior, and retry/cancel state transitions.
- Fake materializer that writes a small manifest and contexts.
- Fake `toscli` runner that records upload calls without network.
- CP mock client from `add-volc-cp-client` for pipeline run lifecycle.
- Backend handler tests for run creation/start/detail/events.
- Frontend component tests for phase timeline and layer progress states when the frontend stack is added.
- One optional manual integration run with real TOS and CP resources.

## Milestones

1. **M1 Foundations**: CP client, config with `.env` + UI credential sources, PostgreSQL schema, mock mode.
2. **M2 Input pipeline**: SWE-bench fork clone/run materializer, generated-directory test mode, TOS upload wrapper, manifest parser.
3. **M3 Orchestration**: CP workspace/pipeline creation, run phases, layer gates, worker pool, CP run/poll/log integration.
4. **M4 Web demo**: settings, run list/detail, SSE progress, image logs.
5. **M5 Polish**: retry/cancel, README, docker-compose, real integration runbook.

## Confirmed Decisions

1. The first implementation clones/runs `https://github.com/sebastian-l0/SWE-bench/tree/feature/materialize-image-contexts` to materialize Dockerfiles. Generated-directory input remains for tests and diagnostics.
2. The first implementation creates CP workspaces and pipelines through CP APIs. It does not require users to pre-create CP resources.
3. Credentials are supported from both `.env` and UI input. UI-entered local configuration takes precedence over `.env`, and all API/log output must redact secrets.
