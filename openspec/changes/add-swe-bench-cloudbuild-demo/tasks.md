# Tasks: add-swe-bench-cloudbuild-demo

## 1. Scope alignment and config

- [x] 1.1 First-demo mode confirmed: clone/run the SWE-bench materializer from `sebastian-l0/SWE-bench` branch `feature/materialize-image-contexts`.
- [x] 1.2 CP mode confirmed: create workspaces and pipelines through CP APIs; do not require pre-created CP resources.
- [x] 1.3 Credential mode confirmed: support both `.env` and UI input, with UI-entered local configuration taking precedence.
- [x] 1.4 Define `.env.example` and config schema for Volc target, TOS bucket/path, dataset, materializer repo/ref, registry namespace, concurrency, CP creation settings, and mock mode.
- [x] 1.5 Add secret redaction helper and tests.

## 2. Backend project foundation

- [x] 2.1 Create backend Go module and server skeleton.
- [x] 2.2 Add PostgreSQL persistence for runs, image builds, attempts, and events; local docker-compose uses `arm64v8/postgres:15`.
- [x] 2.3 Add HTTP routing, JSON error envelope, request validation, and health endpoint.
- [ ] 2.4 Add SSE event bus backed by persisted events.

## 3. Materialization stage

- [ ] 3.1 Implement SWE-bench materializer clone/update flow for `https://github.com/sebastian-l0/SWE-bench` branch `feature/materialize-image-contexts`.
- [ ] 3.2 Implement materializer command runner for the configured dataset/output directory.
- [ ] 3.3 Implement generated-directory input mode for tests and diagnostics that validates `manifest.json` and context paths.
- [ ] 3.4 Persist git repo/ref, command metadata, stdout/stderr tail, output directory, timing, and failure reason.
- [ ] 3.5 Add fake materializer tests and command failure tests.

## 4. TOS upload stage

- [ ] 4.1 Implement `toscli` availability check and version capture.
- [ ] 4.2 Implement upload wrapper to copy generated output to `{bucket}/{parent_path}/{timestamp}/`.
- [ ] 4.3 Persist TOS URI and upload summary.
- [ ] 4.4 Add fake `toscli` tests for success, missing binary, failed upload, and redacted logs.

## 5. Manifest parser and dependency graph

- [ ] 5.1 Parse `base_images`, `env_images`, and `instance_images` from `manifest.json`.
- [ ] 5.2 Normalize each image into `ImageBuild` fields while preserving raw manifest JSON.
- [ ] 5.3 Validate dependency keys and detect missing/duplicate images.
- [ ] 5.4 Add parser fixtures for the expected SWE-bench materializer output.

## 6. CP orchestration

- [ ] 6.1 Integrate the `volc-cp-client` capability from `add-volc-cp-client`.
- [ ] 6.2 Implement CP workspace creation for base/env/instance layers through CP APIs.
- [ ] 6.3 Implement CP service connection and parameterized pipeline creation through CP APIs.
- [ ] 6.4 Implement run phase state machine: materialize -> upload -> create_cp_resources -> base -> env -> instance -> terminal.
- [ ] 6.5 Implement per-layer worker pools and configurable concurrency.
- [ ] 6.6 Implement strict layer gate; optionally add best-effort gate behind config.
- [ ] 6.7 Implement RunPipeline variable mapping from image manifest + TOS prefix + registry.
- [ ] 6.8 Implement polling, status mapping, log lookup metadata, retry, and cancel.
- [ ] 6.9 Add unit tests for CP resource creation, phase transitions, gate behavior, retries, cancellation, and CP failures.

## 7. Backend API

- [ ] 7.1 Implement config APIs: `GET /api/config`, `PUT /api/config`.
- [ ] 7.2 Implement run APIs: create, start, cancel, retry, list, detail, events.
- [ ] 7.3 Implement image APIs: detail, retry, log.
- [ ] 7.4 Add handler tests with fake materializer, fake TOS, and CP mock.

## 8. Web UI

- [ ] 8.1 Create React/Vite frontend with routing and API client.
- [ ] 8.2 Build Settings / New Run page.
- [ ] 8.3 Build Run List page.
- [ ] 8.4 Build Run Detail page with phase timeline, layer cards, counts, failed images, and actions.
- [ ] 8.5 Build Image Detail / Log view.
- [ ] 8.6 Add SSE subscription and reconnect behavior.

## 9. Local demo and documentation

- [ ] 9.1 Add docker-compose or local dev scripts for backend + frontend.
- [ ] 9.2 Add README quickstart for mock mode.
- [ ] 9.3 Add real-run runbook covering SWE-bench materializer, TOS, CP API-created workspace/pipeline resources, and registry assumptions.
- [ ] 9.4 Add sample tiny manifest/context fixture for local demos.

## 10. Validation

- [ ] 10.1 Run backend unit tests.
- [ ] 10.2 Run frontend tests/build once frontend exists.
- [ ] 10.3 Run mock end-to-end demo: generated fixture -> fake upload -> CP mock -> successful run.
- [ ] 10.4 Run `openspec validate add-swe-bench-cloudbuild-demo --strict` if OpenSpec CLI is available.
