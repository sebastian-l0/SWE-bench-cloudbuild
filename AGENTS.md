# AGENTS.md

## Project Mission

SWE-cloudbuild is a local web application for building SWE-bench Docker images through Volcengine cloud services.

The first product target is defined by `PLAN.md` and the OpenSpec change `openspec/changes/add-swe-bench-cloudbuild-demo/`:

1. Clone/run `https://github.com/sebastian-l0/SWE-bench/tree/feature/materialize-image-contexts` to materialize Dockerfile contexts and `manifest.json` for `base_images`, `env_images`, and `instance_images`.
2. Upload generated Dockerfile contexts to TOS under a timestamped prefix.
3. Create Volcengine Continuous Delivery (CP) workspaces and parameterized pipelines through CP APIs.
4. Build images in dependency order: `base -> env -> instance`.
5. Expose a local Web UI for configuration, phase progress, per-image status, logs, retry, and cancel.

## Read These First

Before changing behavior or writing implementation code, read:

- `PLAN.md`
- `openspec/changes/add-swe-bench-cloudbuild-demo/proposal.md`
- `openspec/changes/add-swe-bench-cloudbuild-demo/design.md`
- `openspec/changes/add-swe-bench-cloudbuild-demo/tasks.md`
- `openspec/changes/add-swe-bench-cloudbuild-demo/specs/swe-bench-cloudbuild-demo/spec.md`
- `openspec/changes/add-volc-cp-client/` when touching Volcengine CP API client code

## Current Confirmed Decisions

- First version MUST use the SWE-bench materialization branch: `sebastian-l0/SWE-bench` branch `feature/materialize-image-contexts`.
- First version MUST create CP workspaces and pipelines through CP APIs; do not require manually pre-created CP resources.
- Credentials MUST support both `.env` and UI-entered local configuration. UI-entered values take precedence over `.env`.
- Secrets MUST be redacted from logs, persisted events, and API responses.
- Persistence MUST use PostgreSQL, not SQLite. Local development MUST use Docker image `arm64v8/postgres:15` and backend connection via `DATABASE_URL`.
- The default build gate is strict: all `base` images must succeed before `env`, and all `env` images must succeed before `instance`.
- Generated-directory input is allowed for tests and diagnostics, but it is not the primary first-demo path.

## Expected Architecture

- Backend: Go 1.22, Gin-style HTTP API, GORM or equivalent SQL layer, PostgreSQL.
- Database: PostgreSQL 15 via `arm64v8/postgres:15` in local docker-compose.
- Frontend: React 18 + Vite + TypeScript + Ant Design + React Query.
- Long-running workflow: backend scheduler/worker pool with persisted run state and SSE progress events.
- TOS upload: shell out to `toscli` in the first implementation.
- CP access: use the local Volcengine CP client capability from `add-volc-cp-client`; do not scatter raw CP signing code outside that package.

## Implementation Rules for Agents

1. Follow OpenSpec. If requirements change, update the relevant OpenSpec change before implementation.
2. Keep changes scoped. Do not bundle unrelated refactors.
3. Prefer tests/mocks first for materializer, TOS upload, CP API, scheduler, and state transitions.
4. Do not commit credentials, tokens, AK/SK, `.env`, generated databases, build outputs, or downloaded private artifacts.
5. Do not log secrets. Add redaction at API, event, command-output, and persistence boundaries.
6. Use `rg` / `rg --files` for searching.
7. Respect existing uncommitted changes. Never revert user changes unless explicitly asked.
8. Run verification before claiming completion. At minimum, run the relevant unit tests and `openspec validate <change-id> --strict` when specs changed.

## Database Guidance

- Use PostgreSQL-compatible schema and SQL types from the start.
- Local docker-compose should define a `postgres` service using `arm64v8/postgres:15`.
- Configure the backend with `DATABASE_URL`, for example:
  - `postgres://swe_cloudbuild:swe_cloudbuild@localhost:5432/swe_cloudbuild?sslmode=disable`
- Persist runs, image builds, attempts, and events so workflow status survives backend restarts.
- Avoid SQLite-only assumptions, syntax, pragmas, or tests.

## Security and Credentials

- `.env` and UI-entered local config are both supported.
- UI-entered config overrides `.env` for runtime use.
- API responses should expose only non-secret metadata and masked secret presence, never raw secrets.
- Redact common sensitive keys: `AK`, `SK`, `ACCESS_KEY`, `SECRET_KEY`, `TOKEN`, `PASSWORD`, `DATABASE_URL`, authorization headers, and CP/TOS credentials.

## Useful Commands

```bash
make tdd
make pre-commit
make pre-push
lefthook run pre-commit --no-auto-install
lefthook run pre-push --no-auto-install --force
openspec validate add-swe-bench-cloudbuild-demo --strict
openspec validate add-volc-cp-client --strict
```

Install git hooks with Lefthook:

```bash
make install-hooks
# or: lefthook install --reset-hooks-path
```

Add backend/frontend focused test commands here once the implementation scaffolding exists.
