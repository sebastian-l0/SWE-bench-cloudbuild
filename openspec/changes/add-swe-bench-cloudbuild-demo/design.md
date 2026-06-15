# 设计：SWE-bench CloudBuild 本地演示

## 上下文

SWE-cloudbuild 是一个本地 Web 应用，用于把 SWE-bench Dockerfile context 的生成、上传与云端构建串成一个可观察、可重试、可取消的流程。后续实现以本 OpenSpec change 为唯一需求来源；`PLAN.md` 仅保留为历史草稿，不再作为开发依据。

首版固定使用 SWE-bench materializer 分支：

- 仓库：`https://github.com/sebastian-l0/SWE-bench`
- 分支：`feature/materialize-image-contexts`

生成目录中必须包含 `manifest.json`，并描述三类镜像：

- `base_images`
- `env_images`
- `instance_images`

## 目标 / 非目标

### 目标

- 提供本地 Web UI，引导用户配置并启动一次端到端构建运行。
- 克隆/更新并执行 SWE-bench materializer，生成 Dockerfile context 与 manifest。
- 支持已生成目录输入，作为测试与诊断路径。
- 使用 `toscli` 将生成目录上传到 TOS timestamp prefix。
- 解析 manifest，建立三层镜像依赖图。
- 通过 Volcengine CP API 创建/确保工作区、服务连接和参数化镜像构建流水线。
- 按严格层级闸门执行：所有 base 成功后 env，所有 env 成功后 instance。
- 提供 PostgreSQL 持久化，使运行状态、镜像状态、attempt 和事件在后端重启后可恢复。
- 通过 REST 与 SSE 暴露配置、运行、镜像、日志和进度。
- 支持 mock mode，使没有真实 Volcengine/TOS 凭证时也可本地演示。

### 非目标

- 不执行 SWE-bench evaluation。
- 不支持多租户、远程托管、认证/RBAC。
- 不管理任意 Docker build 系统；仅围绕 SWE-bench materializer 输出。
- 不要求预创建 CP 资源；首版默认通过 API 创建/确保。

## 总体架构

```text
web/ React UI
  -> server HTTP API
      -> config + redaction
      -> PostgreSQL store
      -> materializer runner
      -> toscli uploader
      -> manifest parser
      -> Volcengine CP client + mock client
      -> orchestrator / worker pool
      -> persisted event bus + SSE
```

后端是状态源。前端只负责提交配置、发起操作、订阅 SSE、展示 REST 查询结果。长任务由后端 worker pool 执行，所有关键状态写入 PostgreSQL。

## 技术栈与目录约定

- 后端：Go 1.22，标准库 HTTP 起步；后续可引入 Gin/GORM，但不得破坏已有 API 契约。
- 数据库：PostgreSQL 15，本地 docker-compose 使用 `arm64v8/postgres:15`，连接串通过 `DATABASE_URL`。
- 前端：React 18 + Vite + TypeScript + Ant Design + React Query。
- TOS：首版 shell out 到 `toscli`。
- CP：在本 change 内实现 CP client / mock client；CP 签名、HTTP 调用、mock 不再放到独立 OpenSpec change。

建议目录：

```text
server/
  cmd/server/
  internal/api/
  internal/config/
  internal/model/
  internal/store/
  internal/materializer/
  internal/tosupload/
  internal/manifest/
  internal/volc/
  internal/volc/cp/
  internal/orchestrator/
  migrations/
web/
  src/api/
  src/hooks/
  src/pages/
```

## 配置与凭证

配置项包括：

- Volcengine AK/SK、target、region/endpoint override。
- TOS bucket、parent path。
- SWE-bench dataset 与 split。
- materializer repo/ref。
- 目标 registry namespace。
- CP workspace/pipeline/service connection 创建设置。
- base/env/instance 并发。
- layer gate policy。
- mock mode。

规则：

1. `.env` 与 UI 本地配置都可提供配置。
2. UI 输入优先于 `.env`。
3. API 响应只返回非 secret 配置和 secret presence，不返回原始 secret。
4. 日志、持久化事件、命令输出、错误消息和 API 响应都必须经过 redaction。
5. 常见敏感键：`AK`、`SK`、`ACCESS_KEY`、`SECRET_KEY`、`TOKEN`、`PASSWORD`、`DATABASE_URL`、authorization headers、CP/TOS 凭证。

## 数据模型

核心实体：

```go
type Run struct {
    ID              string
    Name            string
    Status          string // pending/running/success/failed/canceled
    Phase           string // materializing/uploading/preparing_cp/building_base/building_env/building_instance
    Dataset         string
    TOSBucket       string
    TOSPrefix       string
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

type RunAttempt struct {
    ID            string
    ImageBuildID  string
    PipelineRunID string
    Status        string
    LogURL        string
    StartedAt     time.Time
    UpdatedAt     time.Time
    FinishedAt    *time.Time
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

数据库必须使用 PostgreSQL-compatible SQL 类型，避免 SQLite-only 语法。

## Materializer 阶段

支持两种输入：

1. **materializer-command 模式**：克隆/更新 `https://github.com/sebastian-l0/SWE-bench` 到指定 ref，执行 materializer 命令生成输出目录。
2. **generated-directory 模式**：使用已有本地目录，只用于测试和诊断；必须校验 `manifest.json` 和 context 路径。

需要持久化：repo/ref、dataset、命令、输出目录、stdout/stderr tail、开始/结束时间、失败原因。

## TOS 上传阶段

- 检查 `toscli` 是否存在，并记录版本。
- 上传整个生成目录到：`tos://{bucket}/{parent_path}/{yyyyMMddHHmmss}/`。
- 保存 TOS URI 与上传摘要。
- 上传失败时运行失败，且不得触发任何 CP pipeline run。

## Manifest parser 与依赖图

Parser 读取 `manifest.json` 并将条目规范化为 `ImageBuild`：

- layer：`base` / `env` / `instance`
- local key / image name
- target image
- context path
- Dockerfile path
- dependency key：env 指向 base，instance 指向 env
- raw manifest JSON

校验：

- 三层结构存在且格式正确。
- local key 不重复。
- 依赖 key 必须存在。
- context path 与 Dockerfile path 不得越权逃逸输出目录。

## Volcengine CP client

CP client 在本 change 内实现，职责包括：

- target 解析：至少支持 `pre`、`prod-cn`、`prod-sg`、`byteplus-sg`，`prod` 作为 `prod-cn` 别名。
- V4-HMAC-SHA256 签名。
- 统一错误模型：错误码、错误消息、RequestID、HTTP status。
- workspace：create/list/update/delete。
- service connection：create/get/list/update/delete。
- pipeline：create/list/delete，创建接口标注为不稳定契约。
- run：run/list task runs/list pipeline runs/cancel。
- log：分页获取日志与下载 URI。
- mock mode：内存实现 workspace/service connection/pipeline/run/log 生命周期。

CP client 不负责业务重试、限流策略和调度节奏；这些由 orchestrator 控制。

## CP 编排

运行阶段：

1. `materializing_dockerfiles`
2. `uploading_dockerfiles`
3. `preparing_cp_resources`
4. `building_base_images`
5. `building_env_images`
6. `building_instance_images`
7. terminal：`success` / `failed` / `canceled`

编排规则：

- 默认严格闸门：当前层全部成功后才进入下一层。
- 任意当前层镜像永久失败时，后续层停止调度，并按依赖关系标记 skipped。
- 每层有独立并发限制，默认 base=1、env=10、instance=20。
- 每次 CP `RunPipeline` 前创建新的 attempt。
- 如果已有 `LastRunID` 仍在运行，不得重复触发。
- 轮询 CP 状态并映射为本地状态。
- 日志按需通过 CP client 获取，并返回 redacted 内容。
- cancel 需要取消本地 queued work，并尽力调用 CP cancel 运行中 pipeline run。

CP pipeline 变量至少包含：

- `CONTEXT_ROOT`
- `CONTEXT_PATH`
- `DOCKERFILE`
- `TARGET_IMAGE`
- `BASE_IMAGE` 或 `ENV_IMAGE`（适用时）

## API 形态

| Method | Path | 说明 |
|---|---|---|
| `GET` | `/api/config` | 读取非 secret 的有效配置和默认值 |
| `PUT` | `/api/config` | 保存 UI 本地配置 / secret 引用 |
| `POST` | `/api/runs` | 基于配置创建运行 |
| `POST` | `/api/runs/:id/start` | 启动 materialize -> upload -> CP build |
| `POST` | `/api/runs/:id/cancel` | 取消 queued/running work |
| `POST` | `/api/runs/:id/retry` | 重试失败阶段或失败镜像 |
| `GET` | `/api/runs` | 运行列表 |
| `GET` | `/api/runs/:id` | 运行详情和三层摘要 |
| `GET` | `/api/runs/:id/events` | SSE 进度流 |
| `GET` | `/api/images/:id` | 镜像详情 |
| `POST` | `/api/images/:id/retry` | 重试单个失败镜像 |
| `GET` | `/api/images/:id/log` | 获取 CP 日志 |

错误响应统一使用 JSON envelope，例如：

```json
{"error":{"code":"not_found","message":"route not found"}}
```

## Web UI

首版 UI 保持小而完整：

- Settings / New Run：配置凭证状态、dataset、TOS、registry、CP、mock。
- Run List：展示最近 runs 和终态。
- Run Detail：阶段 timeline、三层 card、进度、失败镜像、start/cancel/retry。
- Image Detail / Log：展示 manifest 数据、attempt、CP run/task、日志。
- SSE 自动重连，断线后可通过 REST 恢复当前状态。

## 测试策略

- 配置加载与 redaction 单元测试。
- CP signer、target、client、错误解析和 mock 生命周期测试。
- Manifest parser 与依赖图校验测试。
- Fake materializer 测试成功与命令失败。
- Fake `toscli` 测试成功、缺失 binary、上传失败、日志脱敏。
- Orchestrator 测试阶段迁移、严格闸门、重试、取消、CP 失败。
- API handler 测试 config/run/image/log/events。
- 前端基础 build 和关键组件状态测试。
- mock end-to-end：generated fixture -> fake upload -> CP mock -> success。

## 里程碑

1. **M1 基础工程**：配置/redaction、Go server、PostgreSQL schema、docker-compose、前端脚手架、基础验证。
2. **M2 CP client**：target、签名、HTTP client、workspace/service connection/pipeline/run/log、mock mode。
3. **M3 输入管线**：materializer command、generated-directory、TOS upload、manifest parser。
4. **M4 编排**：CP 资源准备、状态机、worker pool、层级闸门、retry/cancel/log。
5. **M5 Web demo**：settings、run list/detail、SSE、image logs。
6. **M6 文档与演示**：README、真实运行 runbook、mock e2e、示例 fixture。

## 已确认决策

1. 首版使用 `sebastian-l0/SWE-bench` 的 `feature/materialize-image-contexts` 分支。
2. 首版通过 CP API 创建/确保工作区和参数化流水线，不依赖预创建资源。
3. `.env` 与 UI 本地配置都支持，UI 优先。
4. Secret 必须在日志、事件、持久化和 API 响应中脱敏。
5. 持久化使用 PostgreSQL，不使用 SQLite。
6. 默认严格层级闸门。
7. 已生成目录输入仅用于测试和诊断。
