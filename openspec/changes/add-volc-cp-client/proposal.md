# Add Volc CP OpenAPI Client (`add-volc-cp-client`)

## Why

SWE-cloudbuild 需要按照 manifest.json 的 base/env/instance 三层依赖在火山引擎持续交付（CP）上批量纳管 Docker 镜像构建流水线。要实现这一目标，应用必须能够程序化地：创建/查询工作区、把 dockerfile 仓库注册为 CP 服务连接（代码源）、按 YAML Spec 创建参数化的镜像构建流水线、按需 RunPipeline 并轮询状态/拉日志。火山官方文档列出的 OpenAPI 不包含 `CreatePipeline` 与 `CreateServiceConnection`，但根据 `code-pipeline` 仓库内的 `cp-pre-prod-replay` skill 的实践，这两个接口在 pre 与 prod 同 Version 下均可用。本 change 的目标是把这层调用沉淀为一个独立、可单测、可 Mock 的 Go 客户端，作为后续编排器（orchestrator）与 HTTP 层的基础。

## What Changes

- 新增 `server/internal/volc/` Go 包，承载 V4 签名与通用 HTTP 调用（POST + JSON body）。
- 新增 `server/internal/volc/cp/` 子包，按领域拆分接口封装：workspace / serviceconnection / pipeline / run / log。
- 多 target 配置：`pre`、`prod-cn`、`prod-sg`、`byteplus-sg`，每个 target 的 endpoint / region / service / 凭证来源独立；`prod` 作为 `prod-cn` 别名向后兼容。
- 凭证仅通过环境变量传入：主站走 `VOLC_ACCESS_KEY` / `VOLC_SECRET_KEY`；byteplus 国际站走 `VOLC_BYTEPLUS_ACCESS_KEY` / `VOLC_BYTEPLUS_SECRET_KEY`。
- Mock 模式：`VOLC_MOCK=1` 时所有 CP 调用走内置 in-memory 实现，便于本地与 CI 联调，不需要真实凭证。
- 提供活性自检命令 `cmd/cpcheck`（或 `make ping-pre`），调用 `ListWorkspaces` 验证签名/凭证/网络。
- **BREAKING**：无（首次引入）。

## Capabilities

### New Capabilities
- `volc-cp-client`: 火山引擎持续交付 OpenAPI 的 Go 客户端，覆盖签名、target 路由、工作区/服务连接/流水线/运行/日志五组核心接口，以及 Mock 模式。

### Modified Capabilities
（无）

## Impact

- **新增代码**：`server/internal/volc/`（约 1500 LOC）+ `cmd/cpcheck/`（约 50 LOC）+ 单元 / 集成测试。
- **依赖**：仅使用标准库（`crypto/hmac`、`crypto/sha256`、`net/http`、`encoding/json`），不引入第三方 SDK，避免 Volc 官方 Go SDK 的强耦合。
- **配置**：`.env.example` 新增 `VOLC_*` 一组环境变量；`docker-compose.yml` 透传到后端容器。
- **下游依赖**：后续 `add-manifest-batch-orchestrator`、`add-web-api-server` 两个 change 会作为消费者直接 import 本包；接口稳定后不应频繁变更。
- **风险**：CreatePipeline / CreateServiceConnection 不在公开文档中，需在集成测试里覆盖至少一次真实 prod-cn 调用，规避签名/字段错误；同时通过 Mock 模式让上层不被未公开接口阻塞。
