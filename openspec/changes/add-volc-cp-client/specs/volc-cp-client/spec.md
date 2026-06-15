# Volc CP Client Capability

## ADDED Requirements

### Requirement: Multi-target configuration

The client SHALL support four built-in deployment targets — `pre`, `prod-cn`, `prod-sg`, `byteplus-sg` — each with its own endpoint, region, service name, and credential source. The alias `prod` SHALL resolve to `prod-cn`. Per-target environment variables (`VOLC_<NAME>_ENDPOINT`, `_REGION`, `_SERVICE`) SHALL override the built-in defaults so that private deployments can be configured without code changes.

#### Scenario: Resolve known target from defaults
- **WHEN** the caller invokes `volc.NewClient("prod-cn")` without overrides
- **THEN** the client uses endpoint `cp.cn-beijing.volcengineapi.com`, region `cn-beijing`, service `cp`, and reads `VOLC_ACCESS_KEY` / `VOLC_SECRET_KEY` from the environment

#### Scenario: byteplus-sg uses byteplus credentials
- **WHEN** the caller invokes `volc.NewClient("byteplus-sg")`
- **THEN** the client reads `VOLC_BYTEPLUS_ACCESS_KEY` / `VOLC_BYTEPLUS_SECRET_KEY`, signs against region `ap-southeast-1` and service `cp`, and does not attach any extra query parameter beyond `Action` and `Version`

#### Scenario: Reject unknown target
- **WHEN** the caller invokes `volc.NewClient("east-1")`
- **THEN** the call returns an error listing the valid target names

#### Scenario: prod alias resolves to prod-cn
- **WHEN** the caller invokes `volc.NewClient("prod")`
- **THEN** the resulting client behaves identically to `volc.NewClient("prod-cn")`

### Requirement: Signature V4 signing

The client SHALL sign every request using HMAC-SHA256 (Signature V4) with `CredentialScope = <YYYYMMDD>/<region>/<service>/request`, signed headers `content-type;host;x-content-sha256;x-date`, and `Action` + `Version=2023-05-01` included in the canonical query string. The signing key SHALL be derived per request based on the target's region and service.

#### Scenario: Canonical request matches the spec
- **WHEN** the test signs a request with a fixed timestamp, body, and credentials
- **THEN** the resulting `Authorization` header equals the precomputed reference value

#### Scenario: Canonical query contains only Action and Version
- **WHEN** signing a request for any built-in target
- **THEN** the canonical query string equals `Action=<X>&Version=2023-05-01` in lexical order

### Requirement: Workspace operations

The client SHALL expose `CreateWorkspace`, `ListWorkspaces`, `UpdateWorkspace`, and `DeleteWorkspace` covering the official OpenAPI fields (`Name`, `Description`, `Visibility`, `VisibleUsers`, `VisibleUserGroups`). `ListWorkspaces` SHALL provide both a single-page call and an `All` helper that auto-paginates.

#### Scenario: Create workspace returns ID
- **WHEN** the caller invokes `CreateWorkspace(ctx, {Name:"swe-base", Visibility:"Account"})`
- **THEN** the response carries a non-empty `Id` and the workspace is queryable via `ListWorkspaces`

#### Scenario: ListWorkspaces.All paginates until exhaustion
- **WHEN** the API has 250 workspaces and `PageSize` defaults to 100
- **THEN** `ListWorkspaces.All(ctx)` returns all 250 items by issuing 3 paged requests

### Requirement: Service connection operations

The client SHALL expose `CreateServiceConnection`, `GetServiceConnection`, `ListServiceConnections`, `UpdateServiceConnection`, and `DeleteServiceConnection`. Creation SHALL accept the connection `Type` (`github`, `gitlab`, `codeup`, `general-git`, `cr`, …), `Name`, `Url`, and a typed `Auth` payload (token / username+password / SSH key) sufficient to authenticate against the dockerfile repository used by the SWE-cloudbuild pipelines.

#### Scenario: Create GitHub token connection
- **WHEN** the caller invokes `CreateServiceConnection` with `Type:"github"`, `Url:"https://github.com/sebastian-l0/swe-volcs-dockerfils"`, and `Auth.Token` set
- **THEN** the response carries a `connectionId` that can be embedded into a pipeline `Spec` as `loginCredential[].connectionId` or `Resources[].connectionId`

#### Scenario: Get service connection by ID
- **WHEN** the caller invokes `GetServiceConnection(ctx, id)`
- **THEN** the response includes `Type`, `Name`, `Url`, and connection metadata, and the secret fields are not echoed back

### Requirement: Pipeline lifecycle operations

The client SHALL expose `CreatePipeline`, `ListPipelines`, and `DeletePipeline`. `CreatePipeline` SHALL accept `WorkspaceId`, `Name`, `Description`, `Spec` (YAML string), `Parameters[]` (whitelisted to `Key/Value/Secret/Dynamic/Env/OptionValues/Description/UiType`), `Caches[]` (`Key/Path/Description`), `Notification`, and `Resources[]` (referencing service-connection IDs for code checkout). `CreatePipeline` and `CreateServiceConnection` SHALL be marked as unstable contracts in code comments because they are not in the public OpenAPI list.

#### Scenario: CreatePipeline succeeds with minimal body
- **WHEN** the caller passes `{WorkspaceId, Name, Spec}` only
- **THEN** the response includes a non-empty `Id` and the pipeline is visible from `ListPipelines`

#### Scenario: CreatePipeline strips unknown parameter fields
- **WHEN** the caller's `Parameters[i]` carries a non-whitelisted key such as `Foo`
- **THEN** the request body excludes `Foo` while preserving all whitelisted fields

#### Scenario: Pipeline create / list / delete round trip
- **WHEN** the caller creates a pipeline, lists pipelines in the workspace, then deletes the pipeline
- **THEN** the listed pipelines include the new ID before deletion and exclude it afterwards

### Requirement: Pipeline run operations

The client SHALL expose `RunPipeline`, `ListPipelineRuns`, `ListTaskRuns`, and `CancelPipelineRun`. `RunPipeline` SHALL accept `WorkspaceId`, `PipelineId`, optional `Description`, optional `Parameters[]` overrides, and optional `Resources[]` overrides. The response SHALL surface a `PipelineRunId` for status polling.

#### Scenario: RunPipeline returns RunId
- **WHEN** the caller triggers an existing pipeline with overrides
- **THEN** the response carries a non-empty `PipelineRunId` and `ListPipelineRuns` immediately reflects the new run with status `Running` or `Pending`

#### Scenario: ListPipelineRuns filters by run id
- **WHEN** the caller queries with `PipelineRunId=<id>`
- **THEN** at most one record is returned and its status is one of `Running`, `Succeeded`, `Failed`, `Canceled`

#### Scenario: CancelPipelineRun stops a running pipeline
- **WHEN** the caller cancels a `Running` pipeline run
- **THEN** the next `ListPipelineRuns` lookup returns status `Canceled` within 30 seconds

### Requirement: Task log retrieval

The client SHALL expose `GetTaskRunLog` (paginated text fetch) and `GetTaskRunLogDownloadURI` (signed URL for the full log). Callers SHALL be able to discover `TaskRunId` values by listing task runs of a pipeline run.

#### Scenario: Stream log pages until end
- **WHEN** the caller invokes `GetTaskRunLog` with the cursor returned by the previous call
- **THEN** the client returns successive log chunks until `IsEnd=true`

#### Scenario: Download URI is a presigned URL
- **WHEN** the caller invokes `GetTaskRunLogDownloadURI(taskRunId)`
- **THEN** the response includes an HTTPS URL whose query string contains `X-Tos-` or equivalent presign parameters

### Requirement: Mock mode for local and CI use

When the environment variable `VOLC_MOCK=1` is set, `volc.NewClient(target)` SHALL return an in-memory implementation that fulfills the same Go interface as the real client without making any HTTP request. Mock workspaces, service connections, pipelines, and runs SHALL be consistent within a single process so that orchestrators can exercise full lifecycles end-to-end without real credentials.

#### Scenario: Mock satisfies happy-path lifecycle
- **WHEN** the caller (with `VOLC_MOCK=1`) creates a workspace, a pipeline inside it, runs the pipeline, and lists runs
- **THEN** every step succeeds and `ListPipelineRuns` reports `Running` initially, transitioning to `Succeeded` or `Failed` after a configurable simulated duration

#### Scenario: Mock rejects calls referencing unknown IDs
- **WHEN** the caller invokes `RunPipeline` with `PipelineId="does-not-exist"` while in mock mode
- **THEN** the client returns `volc.APIError{Code:"PipelineNotFound"}`

### Requirement: Structured API errors

All non-2xx and `ResponseMetadata.Error`-bearing responses SHALL be surfaced as `volc.APIError{Code, Message, RequestID, HTTPStatus}`. Sentinel errors (`ErrThrottled`, `ErrAuthFailed`, `ErrNotFound`, `ErrInvalidParameter`) SHALL allow callers to switch policy via `errors.Is` without string matching.

#### Scenario: Throttling decoded into sentinel
- **WHEN** the API returns `ResponseMetadata.Error.Code="Throttling"`
- **THEN** `errors.Is(err, volc.ErrThrottled)` returns true and the `RequestID` is preserved

#### Scenario: byteplus error shape (root-level Error) decoded
- **WHEN** byteplus-sg returns a response with `Error.Code="AuthFailure"` at root
- **THEN** the client returns `APIError{Code:"AuthFailure"}` and `errors.Is(err, volc.ErrAuthFailed)` is true

### Requirement: cpcheck self-test command

The repository SHALL provide a `server/cmd/cpcheck` CLI that takes `--target` and prints `RequestId`, the count of returned workspaces, and the first workspace name by calling `ListWorkspaces(PageSize=1)`. Exit code is non-zero on any error, suitable for CI smoke checks and local connectivity validation.

#### Scenario: Successful ping
- **WHEN** the operator runs `cpcheck --target prod-cn` with valid credentials
- **THEN** the process prints `OK request_id=... workspaces=...` and exits with code 0

#### Scenario: Missing credentials
- **WHEN** `VOLC_ACCESS_KEY` is unset and the operator runs `cpcheck --target pre`
- **THEN** the process exits with code 2 and prints a clear hint about the required env vars
