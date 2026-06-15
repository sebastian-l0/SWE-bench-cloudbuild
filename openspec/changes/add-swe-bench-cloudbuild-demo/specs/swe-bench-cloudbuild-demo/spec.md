# swe-bench-cloudbuild-demo Specification

## ADDED Requirements

### Requirement: Local demo configuration

The system SHALL provide local configuration for Volcengine credentials/target, TOS bucket/path, SWE-bench dataset, materializer repository/ref, final image registry namespace, CP workspace/pipeline creation settings, concurrency, and mock mode. Credentials SHALL be loadable from both `.env` and UI-entered local configuration, with UI-entered values taking precedence.

#### Scenario: User prepares a new run

- **GIVEN** the user opens the local web application
- **WHEN** they provide the required dataset, TOS, registry, and CP configuration
- **THEN** the backend SHALL validate the non-secret fields and store a run-ready configuration
- **AND** secrets SHALL be redacted from logs and API responses.

### Requirement: PostgreSQL persistence

The system SHALL persist configuration metadata, runs, image builds, attempts, and events in PostgreSQL. Local development SHALL use the `arm64v8/postgres:15` Docker image and the backend SHALL connect through `DATABASE_URL`.

#### Scenario: Local database starts with docker-compose

- **GIVEN** the user starts the local development stack
- **WHEN** docker-compose starts database services
- **THEN** PostgreSQL SHALL run from the `arm64v8/postgres:15` image
- **AND** the backend SHALL connect using `DATABASE_URL`.

#### Scenario: Workflow state survives restart

- **GIVEN** a run has persisted phase, image, attempt, and event records
- **WHEN** the backend process restarts
- **THEN** the backend SHALL reload run state from PostgreSQL
- **AND** expose the persisted state through the run detail APIs.

### Requirement: Dockerfile materialization

The system SHALL clone/run `https://github.com/sebastian-l0/SWE-bench/tree/feature/materialize-image-contexts` to produce SWE-bench Dockerfile build contexts and a `manifest.json` for `base_images`, `env_images`, and `instance_images` without executing SWE-bench evaluation. Generated-directory input SHALL remain available for tests and diagnostics.

#### Scenario: Generated directory input

- **GIVEN** a local directory containing `manifest.json` and Dockerfile contexts
- **WHEN** the user starts a run in generated-directory mode
- **THEN** the backend SHALL validate the manifest and context paths
- **AND** mark the materialization phase successful without running the external materializer.

#### Scenario: Materializer command input

- **GIVEN** a configured SWE-bench materializer repository/ref and dataset
- **WHEN** the user starts a run in materializer-command mode
- **THEN** the backend SHALL execute the materializer command
- **AND** persist the output directory, command metadata, timing, and stderr/stdout tail
- **AND** fail the run before upload if the command fails.

### Requirement: TOS upload

The system SHALL upload generated Dockerfile contexts to TOS under a timestamped prefix `{parent_path}/{timestamp}/` before triggering CP builds.

#### Scenario: Upload succeeds

- **GIVEN** materialization has produced a valid output directory
- **WHEN** upload starts
- **THEN** the backend SHALL invoke `toscli` to upload the directory to the configured bucket and timestamped prefix
- **AND** persist the resulting TOS URI for downstream CP variables.

#### Scenario: Upload fails

- **GIVEN** materialization has succeeded
- **WHEN** `toscli` is unavailable or returns a non-zero exit code
- **THEN** the backend SHALL mark the run failed
- **AND** SHALL NOT trigger any CP pipeline runs.

### Requirement: Manifest parsing and dependency graph

The system SHALL parse `manifest.json` into image build records with layer, target image, context path, Dockerfile path, and dependency metadata.

#### Scenario: Valid three-layer manifest

- **GIVEN** a manifest with `base_images`, `env_images`, and `instance_images`
- **WHEN** parsing completes
- **THEN** the backend SHALL create image records for all entries
- **AND** env image records SHALL depend on their base image keys when present
- **AND** instance image records SHALL depend on their env image keys when present.

#### Scenario: Invalid dependency

- **GIVEN** a manifest entry references a missing dependency key
- **WHEN** parsing runs
- **THEN** the backend SHALL reject the run before CP builds start
- **AND** report the missing key in the run error.

### Requirement: CP resource creation

The system SHALL create required CP workspaces, service connections, and parameterized image-build pipelines through CP APIs before running image builds.

#### Scenario: Create CP resources for a run

- **GIVEN** materialization, upload, and manifest parsing have succeeded
- **WHEN** CP resource preparation starts
- **THEN** the backend SHALL create or idempotently ensure base/env/instance workspaces
- **AND** create or idempotently ensure parameterized pipelines required for image builds
- **AND** persist the resulting workspace IDs and pipeline IDs before scheduling image builds.

#### Scenario: CP resource creation fails

- **GIVEN** CP resource preparation is running
- **WHEN** workspace or pipeline creation fails permanently
- **THEN** the backend SHALL mark the run failed
- **AND** SHALL NOT trigger image build pipeline runs.

### Requirement: Layered CP orchestration

The system SHALL orchestrate CP image builds in `base -> env -> instance` order with configurable per-layer concurrency.

#### Scenario: Strict successful run

- **GIVEN** a parsed manifest and uploaded TOS prefix
- **WHEN** the run starts CP orchestration in strict mode
- **THEN** all base images SHALL be queued and run first
- **AND** env images SHALL start only after all base images succeed
- **AND** instance images SHALL start only after all env images succeed
- **AND** the run SHALL become successful only after all instance images succeed.

#### Scenario: Strict failure blocks downstream layers

- **GIVEN** strict layer-gate mode
- **WHEN** any image in the current layer fails permanently
- **THEN** the backend SHALL stop scheduling downstream layers
- **AND** mark downstream images skipped where appropriate
- **AND** mark the run failed.

#### Scenario: Pipeline variables

- **GIVEN** an image build is ready to run
- **WHEN** the backend calls CP `RunPipeline`
- **THEN** it SHALL pass variables for context root, context path, Dockerfile, target image, and dependency image values when applicable.

### Requirement: Progress visibility

The system SHALL expose run and image progress through REST APIs and SSE.

#### Scenario: UI subscribes to a run

- **GIVEN** a run is active
- **WHEN** the web UI connects to `/api/runs/:id/events`
- **THEN** the backend SHALL stream phase changes, image status changes, failures, and terminal events
- **AND** the UI SHALL render phase timeline and per-layer progress.

### Requirement: Retry, cancel, and logs

The system SHALL support retrying failed work, canceling active work, and viewing CP logs.

#### Scenario: Retry failed image

- **GIVEN** an image build failed
- **WHEN** the user retries that image
- **THEN** the backend SHALL create a new attempt and call CP `RunPipeline` again
- **AND** update the image and run summaries from the new attempt.

#### Scenario: Cancel active run

- **GIVEN** a run has queued or running image builds
- **WHEN** the user cancels the run
- **THEN** the backend SHALL cancel queued local work
- **AND** call CP cancel for running pipeline runs when possible
- **AND** mark the run canceled after active cancellation settles.

#### Scenario: View image log

- **GIVEN** an image has a CP run/task record
- **WHEN** the user opens the image log view
- **THEN** the backend SHALL fetch logs through the CP client and return redacted log content.
