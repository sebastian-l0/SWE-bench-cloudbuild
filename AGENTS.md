# AGENTS.md

## Project Mission

SWE-cloudbuild 是一个本地 Web 应用，用于通过 Volcengine 云服务构建 SWE-bench Docker 镜像。

后续开发的唯一产品与实现依据是 OpenSpec change：

- `openspec/changes/add-swe-bench-cloudbuild-demo/`

`PLAN.md` 仅保留为历史草稿，不再作为需求或实现依据。

首版目标：

1. 克隆/运行 `https://github.com/sebastian-l0/SWE-bench` 的 `feature/materialize-image-contexts` 分支，生成 `base_images`、`env_images`、`instance_images` 的 Dockerfile contexts 和 `manifest.json`。
2. 将生成的 Dockerfile contexts 上传到 TOS timestamp prefix。
3. 通过 Volcengine Continuous Delivery（CP）API 创建/确保 workspaces、service connections 和参数化 pipelines。
4. 按 `base -> env -> instance` 依赖顺序构建镜像。
5. 暴露本地 Web UI：配置、阶段进度、单镜像状态、日志、重试、取消。

## Read These First

在修改行为或编写实现代码前，必须先阅读：

- `openspec/changes/add-swe-bench-cloudbuild-demo/proposal.md`
- `openspec/changes/add-swe-bench-cloudbuild-demo/design.md`
- `openspec/changes/add-swe-bench-cloudbuild-demo/tasks.md`
- `openspec/changes/add-swe-bench-cloudbuild-demo/specs/swe-bench-cloudbuild-demo/spec.md`

## Current Confirmed Decisions

- 首版 MUST 使用 SWE-bench materialization 分支：`sebastian-l0/SWE-bench` branch `feature/materialize-image-contexts`。
- 首版 MUST 通过 CP API 创建/确保 workspaces、service connections 和 parameterized pipelines；不得要求用户手工预创建 CP 资源。
- CP client、mock client、签名、workspace/pipeline/run/log 封装属于 `add-swe-bench-cloudbuild-demo` change 的实现范围，不再依赖独立 `add-volc-cp-client` change。
- Credentials MUST 同时支持 `.env` 和 UI-entered local configuration。UI-entered values take precedence over `.env`。
- Secrets MUST 从 logs、persisted events、command output、API responses 中脱敏。
- Persistence MUST 使用 PostgreSQL，不使用 SQLite。Local development MUST 使用 Docker image `arm64v8/postgres:15`，backend 通过 `DATABASE_URL` 连接。
- 默认 build gate 是 strict：所有 `base` images 成功后才能进入 `env`，所有 `env` images 成功后才能进入 `instance`。
- Generated-directory input 仅用于 tests 和 diagnostics，不是首版主路径。

## Expected Architecture

- Backend：Go 1.22，HTTP API，PostgreSQL-compatible SQL layer。
- Database：PostgreSQL 15 via `arm64v8/postgres:15` in local docker-compose。
- Frontend：React 18 + Vite + TypeScript + Ant Design + React Query。
- Long-running workflow：backend scheduler/worker pool with persisted run state and SSE progress events。
- TOS upload：首版 shell out to `toscli`。
- CP access：在 `server/internal/volc` / `server/internal/volc/cp` 中集中实现，不要散落 raw CP signing code。

## Implementation Rules for Agents

1. Follow OpenSpec。若需求变化，先更新 `openspec/changes/add-swe-bench-cloudbuild-demo/` 再实现。
2. Keep changes scoped。不要捆绑无关重构。
3. Prefer tests/mocks first for materializer、TOS upload、CP API、scheduler、state transitions。
4. Do not commit credentials、tokens、AK/SK、`.env`、generated databases、build outputs、downloaded private artifacts。
5. Do not log secrets。必须在 API、event、command-output、persistence 边界做 redaction。
6. Use `rg` / `rg --files` for searching。
7. Respect existing uncommitted changes。Never revert user changes unless explicitly asked。
8. Run verification before claiming completion。至少运行相关 unit tests；当 specs changed 时运行 `openspec validate add-swe-bench-cloudbuild-demo --strict`。

## Database Guidance

- 从一开始使用 PostgreSQL-compatible schema 和 SQL types。
- Local docker-compose 定义 `postgres` service，使用 `arm64v8/postgres:15`。
- Backend 使用 `DATABASE_URL`，例如：
  - `postgres://swe_cloudbuild:swe_cloudbuild@localhost:5432/swe_cloudbuild?sslmode=disable`
- 持久化 runs、image builds、attempts、events，使 workflow status 能在 backend restart 后恢复。
- Avoid SQLite-only assumptions、syntax、pragmas、tests。

## Security and Credentials

- `.env` 与 UI-entered local config 都支持。
- UI-entered config overrides `.env` for runtime use。
- API responses 只能暴露非 secret metadata 和 masked secret presence，不能返回 raw secrets。
- Redact common sensitive keys：`AK`、`SK`、`ACCESS_KEY`、`SECRET_KEY`、`TOKEN`、`PASSWORD`、`DATABASE_URL`、authorization headers、CP/TOS credentials。

## Go Toolchain (gvm)

- 后端要求 Go 1.22+（见 `server/go.mod`）。系统默认 `go` 可能是更低版本，运行测试前必须用 gvm 切到兼容版本。
- 加载 gvm 并切换版本：

```bash
source ~/.gvm/scripts/gvm
gvm use go1.26.1   # 或任意 >=1.22 的已装版本；如缺失先 gvm install go1.22
go version          # 确认 >= go1.22
```

- 若系统 `GOROOT` 指向旧版本导致编译失败，运行 go 命令时加 `env -u GOROOT`。
- `scripts/test-backend.sh` 默认使用 PATH 中的 `go`；切换 gvm 版本后再运行，或通过 `GO_BIN="$(command -v go)"` 显式指定。

## Useful Commands

```bash
make tdd
make pre-commit
make pre-push
make test-backend
lefthook run pre-commit --no-auto-install
lefthook run pre-push --no-auto-install --force
openspec validate add-swe-bench-cloudbuild-demo --strict
```

Install git hooks with Lefthook：

```bash
make install-hooks
# or: lefthook install --reset-hooks-path
```
