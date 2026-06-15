# SWE-CloudBuild 实现方案

## 一、目标与场景

基于 [swe-volcs-dockerfils](https://github.com/sebastian-l0/swe-volcs-dockerfils) 中的 `manifest.json`，
按 `base → env → instance` 三层依赖纳管 SWE-bench Docker 镜像在**火山引擎持续交付（CP）**上的并行构建。

核心流程：
1. 解析 `manifest.json`，得到 `base_images` / `env_images` / `instance_images` 三层条目。
2. 在 CP 上为每层创建独立**工作区**（base / env / instance）。
3. 为每个 image 条目在对应工作区下**复用**一条镜像构建流水线（PipelineId + 变量入参）。
4. 调度器按层级触发：`base` 全部完成 → 全量触发 `env` → `env` 全部完成 → 全量触发 `instance`。
5. Web UI 展示三层进度、单条流水线运行状态与日志。

## 二、关键约束（来自 OpenAPI 调研）

| 能力 | OpenAPI | 说明 |
|---|---|---|
| 工作区 CRUD | ✅ `CreateWorkspace` `ListWorkspaces` `UpdateWorkspace` `DeleteWorkspace` | 完整 |
| 流水线**创建** | ❌ 无公开接口 | 需先在控制台/通过 YAML 模板预置 |
| 流水线列表/删除 | ✅ `ListPipelines` `DeletePipeline` | |
| 运行流水线 | ✅ `RunPipeline`（支持入参变量） | 关键 |
| 运行记录 / 任务 | ✅ `ListPipelineRuns` `ListTaskRuns` `CancelPipelineRun` | |
| 日志 | ✅ `GetTaskRunLog` `GetTaskRunLogDownloadURI` | |
| 触发器 | ✅ `CreateTrigger` 等 | 备选编排手段 |

**应对策略**：在每个工作区中预置 **1 条参数化的"通用镜像构建流水线"**（参数：`CONTEXT_PATH`、`DOCKERFILE`、`TARGET_IMAGE`、`BASE_IMAGE`/`ENV_IMAGE` 等），
应用通过 `RunPipeline` + 不同入参驱动 N 次构建，避免对 N 个 image 创建 N 条流水线。

## 三、技术栈

- **后端**：Go 1.22 + Gin + GORM + PostgreSQL（开发环境使用 `arm64v8/postgres:15` Docker 镜像）
- **任务调度**：内置 Goroutine + 有界 worker pool + 状态机（无需引入 Redis/MQ）
- **API 客户端**：自实现火山引擎 V4 签名 HTTP 客户端（`signer/`），调用 CP OpenAPI（`Service=cp`，`Version=2021-04-15`）
- **前端**：React 18 + Vite + TypeScript + Ant Design + React Query
- **协议**：REST + SSE（构建状态实时推送）
- **部署**：根目录 `docker-compose.yml`，`go run` / `pnpm dev` 也能跑

## 四、目录结构

```
SWE-cloudbuild/
├── README.md
├── docker-compose.yml
├── .env.example                # AK/SK / Region / Endpoint / GitRepo
├── server/                     # Go 后端
│   ├── cmd/server/main.go
│   ├── go.mod
│   ├── internal/
│   │   ├── config/             # 环境/配置
│   │   ├── volc/               # 火山 OpenAPI 客户端
│   │   │   ├── signer.go       # V4 签名
│   │   │   ├── client.go       # 通用 GET/POST
│   │   │   └── cp/             # CP 接口封装
│   │   │       ├── workspace.go    # CreateWorkspace / List…
│   │   │       ├── pipeline.go     # ListPipelines / DeletePipeline
│   │   │       ├── run.go          # RunPipeline / ListPipelineRuns / Cancel
│   │   │       └── log.go          # GetTaskRunLog
│   │   ├── manifest/           # manifest.json 解析
│   │   │   └── parser.go
│   │   ├── orchestrator/       # 三层调度器（base→env→instance）
│   │   │   ├── scheduler.go    # 状态机 + 层级闸门
│   │   │   ├── worker.go       # 单条镜像构建任务
│   │   │   └── store.go        # GORM CRUD
│   │   ├── api/                # HTTP handlers
│   │   │   ├── handler_batch.go    # 批次 CRUD / 启动 / 取消
│   │   │   ├── handler_image.go    # 单条镜像状态/日志
│   │   │   ├── handler_workspace.go
│   │   │   └── sse.go              # 进度流
│   │   └── model/              # GORM 实体
│   │       └── entities.go         # Batch / ImageBuild / RunRecord
│   └── migrations/             # PostgreSQL schema migrations
├── web/                        # React 前端
│   ├── package.json
│   ├── vite.config.ts
│   └── src/
│       ├── main.tsx
│       ├── api/client.ts       # axios 封装
│       ├── hooks/useSSE.ts
│       └── pages/
│           ├── BatchList.tsx       # 所有批次（一次 manifest 上传 = 一个批次）
│           ├── BatchDetail.tsx     # 三层卡片 + 进度条 + 失败重试
│           ├── ImageDetail.tsx     # 单镜像 run 记录 + 日志查看
│           └── Settings.tsx        # AK/SK / Region / 工作区/流水线 ID 配置
└── PLAN.md                     # 本文件
```

## 五、数据模型（GORM）

```go
type Batch struct {            // 一次 manifest 上传
    ID            string  // ULID
    Name          string
    ManifestJSON  string  // 原文存档
    Status        string  // pending/running/partial/success/failed/canceled
    BaseTotal     int; BaseDone int; BaseFailed int
    EnvTotal      int; EnvDone  int; EnvFailed  int
    InstTotal     int; InstDone int; InstFailed int
    CreatedAt     time.Time
}

type ImageBuild struct {       // 一个镜像 = 一个构建任务节点
    ID            string
    BatchID       string
    Layer         string       // base / env / instance
    LocalKey      string       // sweb.base.py.x86_64:latest
    TargetImage   string
    ContextPath   string
    Dockerfile    string
    DependsOnKey  string       // env→base, instance→env
    WorkspaceID   string       // 火山工作区
    PipelineID    string       // 复用的通用流水线
    Status        string       // pending/queued/running/success/failed/skipped
    LastRunID     string       // 最近一次 PipelineRun ID
    Attempts      int
    Error         string
    StartedAt     *time.Time
    FinishedAt    *time.Time
}

type RunRecord struct {        // RunPipeline 一次调用的记录
    ID         string  // 火山返回的 PipelineRunID
    ImageID    string
    Status     string  // succeeded/failed/running/canceled
    LogURL     string
    StartedAt  time.Time
    UpdatedAt  time.Time
}
```

## 六、调度状态机

```
Batch:  Pending ─┐
                 ├─► Running(base) ─► Running(env) ─► Running(instance) ─► Success
                 │        │              │                 │
                 │        ▼              ▼                 ▼
                 └────► Failed (任何层全部失败时停止后续层；可手动恢复)

ImageBuild:
    Pending → Queued → Running → Success/Failed → (Failed 可 Retry → Queued)
    Skipped（依赖失败）
```

层级闸门规则（可在 Settings 切换两种策略）：
- **严格**：上层任意 1 个失败即中断后续层（默认）
- **尽力**：上层成功条目对应的下层照常推进（适合大批量场景）

## 七、HTTP 路由

| Method | Path | 说明 |
|---|---|---|
| POST | `/api/batches` | 上传 manifest.json + 选择三层 workspace/pipeline |
| GET  | `/api/batches` | 列表 |
| GET  | `/api/batches/:id` | 详情 + 三层进度 |
| POST | `/api/batches/:id/start` | 启动调度 |
| POST | `/api/batches/:id/cancel` | 全量取消 |
| POST | `/api/batches/:id/retry` | 重跑全部 failed |
| GET  | `/api/batches/:id/events` | SSE 流（状态变更推送） |
| GET  | `/api/images/:id` | 单镜像详情 |
| POST | `/api/images/:id/retry` | 单镜像重试 |
| GET  | `/api/images/:id/log` | 透传 GetTaskRunLog |
| GET  | `/api/workspaces` | 透传 ListWorkspaces |
| POST | `/api/workspaces` | 透传 CreateWorkspace |
| GET  | `/api/workspaces/:id/pipelines` | 透传 ListPipelines |

## 八、火山 OpenAPI 调用要点

- **Endpoint**：`https://open.volcengineapi.com`，`Service=cp`，`Version=2021-04-15`
- **签名**：V4-HMAC-SHA256，header `Authorization: HMAC-SHA256 Credential=…`
- **关键调用**：
  ```
  RunPipeline(WorkspaceId, PipelineId, Variables=[
      {Key:"CONTEXT_PATH", Value:"contexts/base/..."},
      {Key:"DOCKERFILE",   Value:"Dockerfile"},
      {Key:"TARGET_IMAGE", Value:"…/sweb.base.py.x86_64:latest"},
      {Key:"BASE_IMAGE",   Value:""},   // env / instance 时填上一层产物
  ])
  ```
- 轮询 `ListPipelineRuns(PipelineRunId=…)` 拿状态；`status ∈ {Running, Succeeded, Failed, Canceled}`。
- 取日志：`ListTaskRuns` → 拿 `TaskRunId` → `GetTaskRunLog`。
- 轮询节奏：基础 5s，指数退避到 30s，构建中位时长 5–15 分钟。

## 九、关键实现细节

1. **manifest 解析**：按 `base_images / env_images / instance_images` 顺序入库，建立 `local_image_key → ImageBuild.ID` 索引；`env.base_local_image_key` 与 `instance.env_local_image_key` 用于建依赖边。
2. **依赖触发**：worker 完成时给 scheduler 发事件，scheduler 检查所属层是否全部 done，若是则把下一层全部由 `Pending → Queued`。
3. **并发控制**：每层独立 semaphore（默认 base=1, env=10, instance=20，可配置），避免压垮 CP 配额。
4. **幂等**：`RunPipeline` 调用前检查 `ImageBuild.LastRunID` 状态，若仍 Running 则不重复触发。
5. **签名容错**：时间偏差 / 401 时自动 NTP 校时一次后重试一次。
6. **SSE**：调度器把状态变更写入 `chan Event`，每个 SSE 连接订阅一份过滤后的事件。
7. **本地 Mock**：`VOLC_MOCK=1` 时 `volc/cp/*` 走 `mock.go`，每个 RunPipeline 30s 后随机成功/失败，方便联调前端。

## 十、首版交付里程碑

1. **M1 后端骨架**：项目脚手架 + 配置 + 火山签名客户端 + 工作区/流水线 List 透传 + Mock 模式跑通。
2. **M2 manifest + 调度**：解析 + 入库 + 三层调度状态机 + 单元测试覆盖核心闸门。
3. **M3 真实 RunPipeline**：接入真实 OpenAPI、轮询、日志透传。
4. **M4 前端**：批次列表 / 详情三层卡片 / SSE 实时进度 / 单镜像日志查看。
5. **M5 体验**：失败重试、批次取消、Settings 配置页、docker-compose 一键启动。

## 十一、已确认决策 / 后续依赖

- 首版基于 `https://github.com/sebastian-l0/SWE-bench/tree/feature/materialize-image-contexts` 的 materialize 能力生成 Dockerfiles；本地已生成目录仅作为测试/诊断模式。
- 首版通过 CP API 创建工作区和参数化流水线，不依赖用户提前在控制台预创建。
- 凭证同时支持 `.env` 与 UI 输入；UI 本地配置优先级高于 `.env`，日志和 API 响应必须脱敏。
- 三层默认使用 base/env/instance 三个独立工作区，便于权限隔离和进度管理。
- 仍需在实现阶段确认 CP 参数化流水线 YAML 模板、TOS context root 与最终镜像仓库变量的精确字段。

## 十二、OpenSpec 变更追踪

- `openspec/changes/add-volc-cp-client/`：火山引擎持续交付 CP OpenAPI 客户端能力，作为底层 API 基础。
- `openspec/changes/add-swe-bench-cloudbuild-demo/`：根据飞书文档《SWE-bench镜像构建》补充端到端产品目标，覆盖 Dockerfiles materialize、TOS 上传、CP 三层构建编排、本地 Web DEMO 与验证任务。

## 十三、工程 Harness

- `make tdd`：TDD 工作入口，提示红-绿-重构流程并运行当前基础检查。
- `make pre-commit` / `lefthook run pre-commit --no-auto-install`：提交前检查，覆盖 shell 语法、轻量 secret 扫描、OpenSpec strict 校验。
- `make pre-push` / `lefthook run pre-push --no-auto-install --force`：推送前检查，包含 pre-commit 全量检查和 harness 自测。当前仓库无初始 commit 时，本地手动验证需要加 `--force` 或 `--files-from-stdin`，真实 `git push` 时由 Lefthook 自动执行。
- `make install-hooks`：执行 `lefthook install --reset-hooks-path`，启用 Lefthook 管理的 `pre-commit` / `pre-push`。
- 当前基础检查会验证 `add-swe-bench-cloudbuild-demo` 与 `add-volc-cp-client` 两个 OpenSpec change；后续后端/前端脚手架落地后，应把 Go/前端单元测试和构建命令接入 `scripts/check.sh`。
