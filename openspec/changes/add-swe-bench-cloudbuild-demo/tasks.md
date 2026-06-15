# 任务：add-swe-bench-cloudbuild-demo

## 1. 范围对齐与配置

- [x] 1.1 确认首版 materializer：使用 `sebastian-l0/SWE-bench` 的 `feature/materialize-image-contexts` 分支。
- [x] 1.2 确认 CP 模式：通过 CP API 创建/确保工作区和流水线，不要求预创建资源。
- [x] 1.3 确认凭证模式：支持 `.env` 与 UI 本地配置，UI 输入优先。
- [x] 1.4 定义 `.env.example` 与配置 schema：Volc target、TOS bucket/path、dataset、materializer repo/ref、registry namespace、并发、CP 创建设置、mock mode。
- [x] 1.5 增加 secret redaction helper 和测试。

## 2. 后端基础工程

- [x] 2.1 创建后端 Go module 和 server skeleton。
- [x] 2.2 增加 PostgreSQL 持久化基础：runs、image builds、attempts、events；本地 docker-compose 使用 `arm64v8/postgres:15`。
- [x] 2.3 增加 HTTP routing、JSON error envelope、基础请求处理和 health endpoint。
- [ ] 2.4 增加基于持久化事件的 SSE event bus。

## 3. Volcengine CP client（本 change 内实现）

- [ ] 3.1 实现 target 配置：`pre`、`prod-cn`、`prod-sg`、`byteplus-sg`、`prod` 别名、环境变量覆盖。
- [ ] 3.2 实现 V4-HMAC-SHA256 signer，并添加固定时间/请求体金标测试。
- [ ] 3.3 实现通用 CP HTTP client：签名、JSON POST、响应解码、统一 API error。
- [ ] 3.4 实现 workspace API：create/list/update/delete/list-all。
- [ ] 3.5 实现 service connection API：create/get/list/update/delete。
- [ ] 3.6 实现 pipeline API：create/list/delete；create 标注为不稳定契约，并做字段白名单。
- [ ] 3.7 实现 run API：run/list pipeline runs/list task runs/cancel。
- [ ] 3.8 实现 log API：分页日志与下载 URI。
- [ ] 3.9 实现 CP mock client，覆盖 workspace/service connection/pipeline/run/log happy path 与 unknown id 错误。

## 4. Materialization 阶段

- [ ] 4.1 实现 SWE-bench materializer clone/update flow。
- [ ] 4.2 实现 materializer command runner，支持配置 dataset/output directory。
- [ ] 4.3 实现 generated-directory 输入模式，用于测试和诊断，并校验 `manifest.json` 与 context 路径。
- [ ] 4.4 持久化 repo/ref、命令元数据、stdout/stderr tail、输出目录、时间和失败原因。
- [ ] 4.5 增加 fake materializer 测试和命令失败测试。

## 5. TOS 上传阶段

- [ ] 5.1 实现 `toscli` 可用性检查和版本捕获。
- [ ] 5.2 实现上传 wrapper，将生成目录上传到 `{bucket}/{parent_path}/{timestamp}/`。
- [ ] 5.3 持久化 TOS URI 和上传摘要。
- [ ] 5.4 增加 fake `toscli` 测试：成功、缺失 binary、上传失败、日志脱敏。

## 6. Manifest parser 与依赖图

- [ ] 6.1 解析 `base_images`、`env_images`、`instance_images`。
- [ ] 6.2 将每个镜像规范化为 `ImageBuild` 字段，并保留 raw manifest JSON。
- [ ] 6.3 校验 dependency keys、重复 local key、路径逃逸和缺失字段。
- [ ] 6.4 增加 SWE-bench materializer 预期输出 fixture。

## 7. CP 编排

- [ ] 7.1 实现 CP workspace/service connection/pipeline 创建或幂等确保。
- [ ] 7.2 实现运行阶段状态机：materialize -> upload -> prepare_cp -> base -> env -> instance -> terminal。
- [ ] 7.3 实现每层 worker pool 和可配置并发。
- [ ] 7.4 实现严格层级闸门；可选 best-effort 闸门必须置于配置之后。
- [ ] 7.5 实现 RunPipeline 变量映射：manifest + TOS prefix + registry + dependency image。
- [ ] 7.6 实现 polling、状态映射、日志元数据、retry、cancel。
- [ ] 7.7 增加 CP 资源准备、阶段迁移、闸门、retry、cancel、CP 失败的单元测试。

## 8. 后端 API

- [ ] 8.1 完成配置 API：`GET /api/config`、`PUT /api/config`。
- [ ] 8.2 实现运行 API：create、start、cancel、retry、list、detail、events。
- [ ] 8.3 实现镜像 API：detail、retry、log。
- [ ] 8.4 增加 handler 测试，使用 fake materializer、fake TOS、CP mock。

## 9. Web UI

- [x] 9.1 创建 React/Vite frontend 基础脚手架、routing/API client 占位。
- [ ] 9.2 构建 Settings / New Run 页面。
- [ ] 9.3 构建 Run List 页面。
- [ ] 9.4 构建 Run Detail 页面：phase timeline、layer cards、counts、failed images、actions。
- [ ] 9.5 构建 Image Detail / Log 页面。
- [ ] 9.6 增加 SSE subscription 与 reconnect 行为。

## 10. 本地演示与文档

- [x] 10.1 增加基础 docker-compose 和 README quickstart。
- [ ] 10.2 补充 mock mode 端到端 quickstart。
- [ ] 10.3 增加真实运行 runbook：SWE-bench materializer、TOS、CP API-created resources、registry 假设。
- [ ] 10.4 增加 tiny manifest/context fixture，用于本地 demo。

## 11. 验证

- [x] 11.1 后端基础单元测试通过。
- [ ] 11.2 前端依赖安装后运行 frontend build/test。
- [ ] 11.3 跑通 mock end-to-end：generated fixture -> fake upload -> CP mock -> success。
- [ ] 11.4 OpenSpec strict validation 通过。
