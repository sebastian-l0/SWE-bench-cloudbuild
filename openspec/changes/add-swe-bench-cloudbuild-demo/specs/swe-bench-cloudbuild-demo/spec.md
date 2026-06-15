# swe-bench-cloudbuild-demo 规范

## ADDED Requirements

### Requirement: 本地演示配置

系统 SHALL 提供本地配置能力，覆盖 Volcengine 凭证/target、TOS bucket/path、SWE-bench dataset、materializer repository/ref、目标镜像仓库 namespace、CP workspace/pipeline/service connection 创建设置、并发和 mock mode。凭证 SHALL 可从 `.env` 与 UI 本地配置加载，且 UI 本地配置优先。

#### Scenario: 用户准备新运行

- **GIVEN** 用户打开本地 Web 应用
- **WHEN** 用户提供 dataset、TOS、registry、CP 和并发配置
- **THEN** 后端 SHALL 校验非 secret 字段并保存可运行配置
- **AND** secret SHALL 从日志、事件和 API 响应中脱敏。

#### Scenario: UI 配置覆盖环境变量

- **GIVEN** `.env` 和 UI 本地配置都提供同一配置项
- **WHEN** 后端解析有效配置
- **THEN** UI 本地配置 SHALL 优先于 `.env`
- **AND** API 响应 SHALL 只暴露 secret presence，不暴露原始 secret。

### Requirement: PostgreSQL 持久化

系统 SHALL 使用 PostgreSQL 持久化配置元数据、runs、image builds、attempts 和 events。本地开发 SHALL 使用 `arm64v8/postgres:15` Docker 镜像，后端 SHALL 通过 `DATABASE_URL` 连接。

#### Scenario: docker-compose 启动本地数据库

- **GIVEN** 用户启动本地开发栈
- **WHEN** docker-compose 启动数据库服务
- **THEN** PostgreSQL SHALL 使用 `arm64v8/postgres:15` 镜像运行
- **AND** 后端 SHALL 使用 `DATABASE_URL` 连接。

#### Scenario: 工作流状态在重启后恢复

- **GIVEN** 一个 run 已持久化 phase、image、attempt 和 event 记录
- **WHEN** 后端进程重启
- **THEN** 后端 SHALL 从 PostgreSQL 重新加载 run state
- **AND** 通过 run detail API 暴露持久化状态。

### Requirement: Dockerfile materialization

系统 SHALL 克隆/运行 `https://github.com/sebastian-l0/SWE-bench/tree/feature/materialize-image-contexts`，生成 SWE-bench Dockerfile build contexts 和包含 `base_images`、`env_images`、`instance_images` 的 `manifest.json`，且不执行 SWE-bench evaluation。Generated-directory 输入 SHALL 保留为测试和诊断能力。

#### Scenario: 使用已生成目录输入

- **GIVEN** 本地目录包含 `manifest.json` 和 Dockerfile contexts
- **WHEN** 用户以 generated-directory 模式启动 run
- **THEN** 后端 SHALL 校验 manifest 和 context paths
- **AND** 不运行外部 materializer，直接将 materialization phase 标记为成功。

#### Scenario: 使用 materializer 命令输入

- **GIVEN** 已配置 SWE-bench materializer repository/ref 和 dataset
- **WHEN** 用户以 materializer-command 模式启动 run
- **THEN** 后端 SHALL 执行 materializer 命令
- **AND** 持久化输出目录、命令元数据、时间和 stderr/stdout tail
- **AND** 若命令失败，run SHALL 在上传前失败。

### Requirement: TOS 上传

系统 SHALL 在触发 CP 构建前，将生成的 Dockerfile contexts 上传到 TOS timestamp prefix `{parent_path}/{timestamp}/`。

#### Scenario: 上传成功

- **GIVEN** materialization 已生成有效输出目录
- **WHEN** 上传开始
- **THEN** 后端 SHALL 调用 `toscli` 将目录上传到配置的 bucket 和 timestamp prefix
- **AND** 持久化 TOS URI，供后续 CP pipeline 变量使用。

#### Scenario: 上传失败

- **GIVEN** materialization 已成功
- **WHEN** `toscli` 不可用或返回非零退出码
- **THEN** 后端 SHALL 将 run 标记为 failed
- **AND** 不得触发任何 CP pipeline run。

### Requirement: Manifest parsing 和依赖图

系统 SHALL 将 `manifest.json` 解析为 image build records，字段包括 layer、target image、context path、Dockerfile path 和 dependency metadata。

#### Scenario: 有效三层 manifest

- **GIVEN** manifest 包含 `base_images`、`env_images` 和 `instance_images`
- **WHEN** 解析完成
- **THEN** 后端 SHALL 为所有条目创建 image records
- **AND** env image records SHALL 在存在依赖时指向 base image keys
- **AND** instance image records SHALL 在存在依赖时指向 env image keys。

#### Scenario: 无效依赖

- **GIVEN** manifest 条目引用不存在的 dependency key
- **WHEN** 解析运行
- **THEN** 后端 SHALL 在 CP 构建开始前拒绝该 run
- **AND** 在 run error 中报告缺失 key。

#### Scenario: 路径逃逸被拒绝

- **GIVEN** manifest 中 context path 或 Dockerfile path 试图逃逸生成目录
- **WHEN** 解析运行
- **THEN** 后端 SHALL 拒绝该 manifest
- **AND** 不得执行 TOS 上传或 CP 构建。

### Requirement: Volcengine CP client

系统 SHALL 在本 change 内提供 Volcengine CP API client 与 mock client，覆盖 target 解析、V4 签名、统一错误、workspace、service connection、pipeline、run 和 log 操作。

#### Scenario: 解析内置 target

- **WHEN** 调用方选择 `prod-cn` target
- **THEN** client SHALL 使用对应 endpoint、region、service 和凭证来源
- **AND** `prod` SHALL 作为 `prod-cn` 别名。

#### Scenario: 签名请求

- **GIVEN** 固定时间、请求体和测试凭证
- **WHEN** client 签名 CP 请求
- **THEN** Authorization header SHALL 与预期金标一致。

#### Scenario: Mock 生命周期

- **GIVEN** mock mode 已启用
- **WHEN** orchestrator 创建 workspace、创建 pipeline、运行 pipeline 并查询 run
- **THEN** mock client SHALL 返回一致的内存对象状态
- **AND** 不发出真实 HTTP 请求。

### Requirement: CP 资源创建

系统 SHALL 在运行镜像构建前，通过 CP API 创建或幂等确保所需 workspace、service connection 和参数化 image-build pipeline。

#### Scenario: 为 run 准备 CP 资源

- **GIVEN** materialization、upload 和 manifest parsing 已成功
- **WHEN** CP resource preparation 开始
- **THEN** 后端 SHALL 创建或幂等确保 base/env/instance workspaces
- **AND** 创建或幂等确保镜像构建所需参数化 pipelines
- **AND** 在调度镜像构建前持久化 workspace IDs 和 pipeline IDs。

#### Scenario: CP 资源创建失败

- **GIVEN** CP resource preparation 正在运行
- **WHEN** workspace、service connection 或 pipeline 创建永久失败
- **THEN** 后端 SHALL 将 run 标记为 failed
- **AND** 不得触发 image build pipeline runs。

### Requirement: 分层 CP 编排

系统 SHALL 按 `base -> env -> instance` 顺序编排 CP 镜像构建，并支持可配置的每层并发。

#### Scenario: 严格模式成功运行

- **GIVEN** 已解析 manifest 并上传 TOS prefix
- **WHEN** run 在 strict mode 启动 CP 编排
- **THEN** 所有 base images SHALL 先进入队列并运行
- **AND** env images SHALL 仅在所有 base images 成功后开始
- **AND** instance images SHALL 仅在所有 env images 成功后开始
- **AND** run SHALL 在所有 instance images 成功后变为 success。

#### Scenario: 严格模式失败阻断下游层

- **GIVEN** strict layer-gate mode
- **WHEN** 当前层任意 image 永久失败
- **THEN** 后端 SHALL 停止调度下游层
- **AND** 按依赖关系将下游 images 标记为 skipped
- **AND** 将 run 标记为 failed。

#### Scenario: Pipeline 变量

- **GIVEN** 某 image build 已准备运行
- **WHEN** 后端调用 CP `RunPipeline`
- **THEN** 请求 SHALL 包含 context root、context path、Dockerfile、target image 和适用的 dependency image 变量。

### Requirement: 进度可见性

系统 SHALL 通过 REST API 和 SSE 暴露 run 与 image 进度。

#### Scenario: UI 订阅运行进度

- **GIVEN** run 处于 active 状态
- **WHEN** Web UI 连接 `/api/runs/:id/events`
- **THEN** 后端 SHALL 流式发送 phase changes、image status changes、failures 和 terminal events
- **AND** UI SHALL 渲染 phase timeline 和每层进度。

#### Scenario: 后端重启后 SSE 可恢复

- **GIVEN** run events 已持久化
- **WHEN** UI 在后端重启后重新订阅 events
- **THEN** 后端 SHALL 能从持久化事件恢复并继续发送后续事件
- **AND** UI SHALL 可通过 REST detail API 补齐当前状态。

### Requirement: Retry、cancel 和 logs

系统 SHALL 支持重试失败 work、取消 active work 和查看 CP logs。

#### Scenario: 重试失败镜像

- **GIVEN** image build 已失败
- **WHEN** 用户重试该 image
- **THEN** 后端 SHALL 创建新的 attempt 并再次调用 CP `RunPipeline`
- **AND** 从新 attempt 更新 image 和 run 摘要。

#### Scenario: 取消 active run

- **GIVEN** run 包含 queued 或 running image builds
- **WHEN** 用户取消 run
- **THEN** 后端 SHALL 取消 queued local work
- **AND** 对 running pipeline runs 尽力调用 CP cancel
- **AND** 在 active cancellation settle 后将 run 标记为 canceled。

#### Scenario: 查看镜像日志

- **GIVEN** image 有 CP run/task record
- **WHEN** 用户打开 image log view
- **THEN** 后端 SHALL 通过 CP client 拉取日志
- **AND** 返回 redacted log content。
