# 新增 SWE-bench CloudBuild 本地演示（`add-swe-bench-cloudbuild-demo`）

## 背景与目标

本项目要提供一个本地启动的 Web 应用，将 SWE-bench 镜像构建输入转化为火山引擎云端构建流程。后续开发以本 OpenSpec change 为准，不再依赖 `PLAN.md`，也不再依赖独立的 `add-volc-cp-client` change。

首版端到端流程：

1. 使用 `https://github.com/sebastian-l0/SWE-bench` 的 `feature/materialize-image-contexts` 分支生成 `base_images`、`env_images`、`instance_images` 三层 Dockerfile context 和 `manifest.json`，不在本地执行 SWE-bench evaluation。
2. 将生成的 Dockerfile context 上传到 TOS：`{parent_path}/{timestamp}/`。
3. 通过火山引擎持续交付（CP）API 创建/确保工作区与参数化镜像构建流水线。
4. 按依赖顺序构建镜像：`base -> env -> instance`。
5. 在本地 Web UI 展示配置、阶段进度、每个镜像状态、日志、重试与取消。

## 变更内容

- 新增本地 SWE-bench CloudBuild 演示能力。
- 新增配置能力：Volcengine AK/SK、CP target、TOS bucket/path、SWE-bench dataset、materializer repo/ref、目标镜像仓库 namespace、并发、CP 资源创建设置、mock mode。
- 凭证来源支持 `.env` 与 UI 本地配置；UI 输入优先于 `.env`；日志、事件、持久化输出和 API 响应必须脱敏。
- 后端负责 materialize、TOS 上传、manifest 解析、CP 资源准备、三层构建编排、状态持久化和 SSE 进度事件。
- 前端提供设置/新建运行、运行列表、运行详情、镜像详情/日志等页面。
- PostgreSQL 作为唯一持久化数据库；本地开发使用 `arm64v8/postgres:15`。
- CP 客户端能力纳入本 change 的后端实现范围，不再通过独立 OpenSpec change 管理。

## 能力范围

### 新增能力

- `swe-bench-cloudbuild-demo`：本地 Web 应用与后端编排能力，用于生成 SWE-bench Dockerfile context、上传 TOS，并通过 Volcengine CP 按依赖顺序构建镜像。

### 本 change 内部能力

- 本地配置与 secret redaction。
- PostgreSQL schema 与运行状态持久化。
- SWE-bench materializer runner。
- `toscli` 上传封装。
- Manifest parser 与依赖图校验。
- Volcengine CP API client、mock client、workspace/pipeline/run/log 封装。
- 调度器、worker pool、SSE 事件。
- 本地 Web UI。

## 影响

- 增加后端模块：配置、redaction、数据库模型/迁移、materializer、TOS uploader、manifest parser、CP client、orchestrator、HTTP/SSE API。
- 增加前端模块：设置/新建运行、运行列表、运行详情、镜像详情、日志视图。
- 增加 `.env.example`、docker-compose、README/runbook、mock mode。
- 增加测试：配置/redaction、manifest、TOS、CP client/mock、调度状态机、API handler、前端基础构建。

## 非目标

- 不运行 SWE-bench evaluation。
- 不在本地真实构建 Docker 镜像；本地构建仅可作为 mock/开发诊断能力。
- 不实现多租户 SaaS、用户认证、RBAC 或远程托管。
- 不要求用户预先在 CP 控制台手动创建工作区或流水线；首版应通过 CP API 创建/确保资源。
