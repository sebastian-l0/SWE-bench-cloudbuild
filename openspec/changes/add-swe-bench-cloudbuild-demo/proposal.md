# Add SWE-bench CloudBuild Demo (`add-swe-bench-cloudbuild-demo`)

## Why

The project goal is to provide a local web application that turns SWE-bench image build inputs into a staged cloud build workflow on Volcengine. The source product note defines an end-to-end flow:

1. Use the materialization capability from a SWE-bench fork to generate Dockerfiles for `base`, `env`, and `instance` images instead of building/evaluating them locally.
2. Upload the generated Dockerfile contexts to a configured TOS bucket/path, using a timestamped parent directory for each run.
3. Create and run Volcengine Continuous Delivery (CP) workspaces / image-build pipelines in dependency order: `base_images` first, then `env_images`, then `instance_images`.
4. Show a basic product flow in a local web UI: Dockerfile generation, Dockerfile upload, and the three image build phases.

The current OpenSpec change `add-volc-cp-client` covers only the CP OpenAPI client foundation. This change captures the full demo/product scope so implementation work can be planned as coherent milestones.

## What Changes

- Add an end-to-end capability for a local SWE-bench CloudBuild demo.
- Add user configuration for Volcengine AK/SK, TOS bucket/path, SWE-bench dataset, Dockerfile materialization repository/ref, and final container registry namespace. Credentials are supported from both `.env` and UI-entered local configuration, with UI values taking precedence and all responses/logs redacted. Credentials are supported from both `.env` and UI-entered local configuration, with UI values taking precedence and all responses/logs redacted.
- Add a backend workflow that clones/runs `https://github.com/sebastian-l0/SWE-bench/tree/feature/materialize-image-contexts` to materialize Dockerfiles, uploads them to TOS with `toscli`, parses the generated `manifest.json`, creates CP workspaces/pipelines through CP APIs, then orchestrates CP image builds in `base -> env -> instance` order.
- Add a local web UI that shows configuration, phase progress, per-layer progress, per-image build status, failures, retries, cancellation, and logs.
- Reuse `add-volc-cp-client` as the CP client dependency instead of redefining low-level CP signing and API details here.

## Capabilities

### New Capabilities

- `swe-bench-cloudbuild-demo`: local web application and backend orchestration for materializing SWE-bench Dockerfiles, uploading them to TOS, and building images through Volcengine CP in dependency order.

### Related / Dependent Capabilities

- `volc-cp-client` from `add-volc-cp-client`: CP workspace, service connection, pipeline, run, and log API client.

## Impact

- Adds backend modules for configuration, materialization command execution, TOS upload, manifest parsing, orchestration, persistence, and HTTP/SSE APIs.
- Adds frontend pages for setup, run overview, batch details, image details, and logs.
- Adds PostgreSQL persistence so workflow state survives page refreshes and backend restarts. Local development uses the `arm64v8/postgres:15` Docker image.
- Adds `.env.example`, README/runbook, and mock mode so the demo can be exercised without real Volcengine credentials.
- Introduces operational assumptions around local `toscli`, dataset access, GitHub repository access, and CP pipeline templates.

## Non-goals

- Do not run SWE-bench evaluation in this application.
- Do not build Docker images locally except in optional development mocks.
- Do not implement a multi-tenant hosted SaaS; this is a local single-user demo.
- Do not depend on pre-created CP workspaces or pipelines for the first runnable demo; the application should create them through CP APIs.
