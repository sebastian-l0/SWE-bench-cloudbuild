# Design: Volc CP OpenAPI Client

## Context

火山引擎持续交付（CP）OpenAPI 通过 `https://<endpoint>/?Action=<X>&Version=2023-05-01` 暴露能力，使用火山特有的 V4-HMAC-SHA256 签名（参考 https://www.volcengine.com/docs/6461/1277764）。CP 在以下部署上提供同一组 Action：

| target | endpoint | region | service | 凭证来源 |
|---|---|---|---|---|
| pre | `cp-pre.cn-beijing.volcengineapi.com` | `cn-beijing` | `cp_pre` | `VOLC_ACCESS_KEY` / `VOLC_SECRET_KEY` |
| prod-cn | `cp.cn-beijing.volcengineapi.com` | `cn-beijing` | `cp` | `VOLC_ACCESS_KEY` / `VOLC_SECRET_KEY` |
| prod-sg | `cp.ap-southeast-1.volcengineapi.com` | `ap-southeast-1` | `cp` | `VOLC_ACCESS_KEY` / `VOLC_SECRET_KEY` |
| byteplus-sg | `cp.ap-southeast-1.byteplusapi.com` | `ap-southeast-1` | `cp` | `VOLC_BYTEPLUS_ACCESS_KEY` / `VOLC_BYTEPLUS_SECRET_KEY` |

`region` 进入签名 CredentialScope；service 与 endpoint 也需跟随 target 切换。`CreatePipeline` 与 `CreateServiceConnection` 在公开文档中未列出，但在 pre 与 prod 上同 Version 可用，需要按 `cp_api.py` 的实现经验对待 —— 字段名首字母大写（PascalCase）、JSON body POST。

参考实现：`/Users/bytedance/go/src/code.byted.org/infcp/code-pipeline/.trae/skills/cp-pre-prod-replay/scripts/cp_api.py`、`replay_to_prod.py`、`run_pipelines.py`。该 skill 已经在生产/预发环境上验证过签名与 CreatePipeline 调用路径。

SWE-cloudbuild 是全新仓库，`server/` 目录将作为独立 Go module（`github.com/sebastian-l0/swe-cloudbuild/server`，路径以 git 远端为准）。本 change 仅完成客户端层；上层调度器与 HTTP 层在后续 change 实现。

## Goals / Non-Goals

**Goals**
- 提供一个零第三方依赖的 Go 客户端，能稳定调用 CP 五组核心接口（workspace / serviceconnection / pipeline / run / log）。
- 通过 target 抽象隔离 endpoint / region / service / 凭证差异，调用方只关心 target 名。
- Mock 模式（`VOLC_MOCK=1`）：让上层 orchestrator 与 web 在没有真实 AK/SK 时也能跑端到端流程。
- 单元测试覆盖签名计算、target 路由、错误解码；集成测试可选（依赖真实凭证，CI 默认跳过）。

**Non-Goals**
- 不封装所有 CP OpenAPI（触发器、应用交付、托管应用等本期不涉及）。
- 不内置 retry / 限流 / 熔断中间件，由调用方在 orchestrator 层根据业务节奏决定。
- 不引入 protobuf / Thrift IDL，所有请求 / 响应使用 `map[string]any` 或场景化的 struct（对核心字段强类型，对透传字段弱类型 RawMessage）。
- 不实现 Volc 官方 Go SDK 集成；标准库已足够，避免间接依赖与 vendoring。

## Decisions

### D1：自实现 V4 签名而非引入官方 SDK
- 选择：手写 `signer.go`，复用 `cp_api.py` 的算法骨架。
- 备选：`github.com/volcengine/volc-sdk-golang`。
- 理由：官方 SDK 体积大、依赖多，且对 pre 环境与 byteplus-sg 的支持需要额外配置；签名算法本身只有 ~80 行，独立实现易测、易维护。

### D2：Target 抽象 + 环境变量优先级
```go
type Target struct {
    Name     string
    Endpoint string
    Region   string
    Service  string
    Scheme   CredentialScheme
}
```
内置 4 个 target；通过 `VOLC_<NAME_UPPERCASE_UNDERSCORE>_ENDPOINT` 等环境变量覆盖，便于私有化场景。`prod` 作为 `prod-cn` 别名。

### D3：包结构
```
server/internal/volc/
├── signer.go             # V4 签名（导出 Sign 方法，方便单测）
├── client.go             # *Client：Do(action, body) -> (raw json, error)
├── target.go             # Target 注册表 + 环境变量解析
├── errors.go             # APIError 类型，承载 Code/Message/RequestId
├── mock.go               // VOLC_MOCK=1 时启用 in-memory 实现
└── cp/
    ├── workspace.go      # CreateWorkspace / ListWorkspaces / Update / Delete
    ├── serviceconn.go    # CreateServiceConnection / Get / List / Update / Delete
    ├── pipeline.go       # CreatePipeline / ListPipelines / DeletePipeline
    ├── run.go            # RunPipeline / ListPipelineRuns / ListTaskRuns / Cancel
    └── log.go            # GetTaskRunLog / GetTaskRunLogDownloadURI
```
`cp/` 子包持有 `*volc.Client`，每个 Action 函数签名形如 `func ListPipelines(ctx, c, req) (*Resp, error)`，输入输出 struct 仅声明业务用到的字段，未知字段透传 `Extra json.RawMessage`。

### D4：Mock 模式实现
- `volc.NewClient(target)` 内部判断 `os.Getenv("VOLC_MOCK") == "1"`：true 时返回 `*mockClient`，假数据保存在进程内 map。
- Mock 实现仅满足 happy path：CreateWorkspace 返回 `ws-mock-<rand>`、CreatePipeline 返回 `pl-mock-<rand>`、RunPipeline 返回 `run-mock-<rand>` 并在 30s 后随机成功/失败。
- Mock 不模拟限流 / 错误码；这一点上层应自行注入 fault injection。
- 单元测试不依赖 `VOLC_MOCK`，而是直接构造 `volc.Client{HTTP: roundTripperStub}`。

### D5：错误模型
- 火山主站把 Error 放在 `ResponseMetadata.Error`；byteplus 国际站直接挂在 root。客户端统一解析为 `APIError{Code, Message, RequestID, HTTPStatus}`。
- `errors.Is(err, volc.ErrThrottled)` 等哨兵错误用于上层根据 Code 做策略（`Throttling`/`InvalidParameter`/`AuthFailure` 等），而不暴露字符串匹配。

### D6：CreatePipeline 字段最小集
来自 `replay_to_prod.py` 的实践：
- 必填：`WorkspaceId`、`Name`、`Spec`（YAML 流水线编排）
- 可选：`Description`、`Parameters[]`、`Caches[]`、`Notification`、`Resources[]`（关联代码源 ServiceConnection）
- Parameters 元素白名单：`Key/Value/Secret/Dynamic/Env/OptionValues/Description/UiType`
- Caches 元素白名单：`Key/Path/Description`
本 change 用 `CreatePipelineRequest` 结构强类型暴露上述字段；`Spec` 直接 `string`，由上层 orchestrator 模板化生成。

### D7：CreateServiceConnection 字段
依据火山持续交付控制台与 `cp-pre-prod-replay` skill 中对 `connectionId` 的复用经验：
- 必填：`Name`、`Type`（`github` / `gitlab` / `codeup` / `general-git` / `cr` 等）、`Auth`（按 Type 不同传 `Token` / `Username+Password` / `SSHKey`）
- 可选：`Description`、`Url`（仓库地址）
- 创建后返回 `Id`（`connectionId`），后续 `RunPipeline` 的 Spec 中 `Resources` 字段引用此 Id 来 checkout 仓库。

### D8：`cmd/cpcheck` 自检
小型 CLI，参数 `--target pre|prod-cn|prod-sg|byteplus-sg`，调用 `ListWorkspaces(PageSize=1)`，打印 `RequestId / WorkspaceCount / 第一个 Workspace.Name`。失败时退出码非零，给 CI 用。

## Risks / Trade-offs

| 风险 | 缓解 |
|---|---|
| CreatePipeline / CreateServiceConnection 是未公开 API，字段可能变化 | 增加 `cp_pipeline_create_test.go` 集成测试（标 build tag `integration`，CI 手动触发），并在 README 标注接口为 unstable contract。 |
| byteplus-sg 凭证与主站不同，签名易混 | Target 配置里 `Scheme` 单独枚举，凭证解析按 `Scheme` 分支；`signer_test.go` 覆盖两种凭证来源的 case。 |
| Mock 模式与真实 API 行为差距大 | Mock 仅覆盖 happy path，错误注入由上层 orchestrator 在测试中显式构造；并通过集成测试与生产对账。 |
| AK/SK 通过环境变量传递，存在被误 commit 风险 | `.gitignore` 加 `.env`、`.env.local`；`make ping-pre` 等命令仅读环境变量；codegen 不允许把 AK/SK 写入文件。 |
| 单条调用同步轮询会阻塞调度器 | 客户端层只暴露同步调用；轮询 / 并发由上层 orchestrator 用 worker pool 处理。本层保持 stateless。 |

## Migration Plan

无遗留代码需迁移。M1 完成后，`server/internal/volc` 即为 SWE-cloudbuild 的 CP 调用唯一入口；后续若需替换为官方 SDK，可在 `volc.Client` 接口背后切换。

## Open Questions

1. byteplus-sg 是否在本期就需要完整支持？默认实现并提供配置，但 CI 集成测试只跑 prod-cn。
2. `CreateServiceConnection` 是否需要支持镜像仓库类型（CR）作为 push 目标？M1 仅支持 git 类型代码源；CR 凭据复用控制台预置即可，下个 change 再扩展。
3. `Spec` YAML 模板是否在客户端层提供 helper？倾向不在客户端层，留给 orchestrator 持有 template；客户端只接受 string。
