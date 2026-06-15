# Tasks: add-volc-cp-client

## 1. 项目骨架

- [ ] 1.1 在 `server/` 下初始化 Go module（`go mod init <module-path>`），新增 `.gitignore` 排除 `.env`、`bin/`、`coverage.out`
- [ ] 1.2 创建目录骨架：`server/internal/volc/{signer,client,target,errors,mock}.go`、`server/internal/volc/cp/`、`server/cmd/cpcheck/`
- [ ] 1.3 在仓库根 `.env.example` 中加入 `VOLC_ACCESS_KEY` / `VOLC_SECRET_KEY` / `VOLC_BYTEPLUS_ACCESS_KEY` / `VOLC_BYTEPLUS_SECRET_KEY` / `VOLC_MOCK` 五个变量与说明

## 2. 签名与目标抽象

- [ ] 2.1 实现 `signer.go`：`Sign(req *http.Request, body []byte, ak, sk, region, service string, now time.Time)`，导出供单测
- [ ] 2.2 编写 `signer_test.go`：固定时间戳/凭证/请求体的金标用例（与 cp_api.py 对照），断言 `Authorization` 头逐字符相等
- [ ] 2.3 实现 `target.go`：`Target` 结构体 + 4 个内置 target（pre/prod-cn/prod-sg/byteplus-sg），别名 `prod -> prod-cn`，环境变量覆盖逻辑
- [ ] 2.4 编写 `target_test.go`：默认值、别名、环境变量覆盖、未知 target、byteplus AccountID 行为

## 3. 通用 HTTP 客户端

- [ ] 3.1 实现 `client.go`：`Client.Do(ctx, action, body)`，POST + JSON、附加签名头、解析响应、统一错误模型
- [ ] 3.2 实现 `errors.go`：`APIError{Code, Message, RequestID, HTTPStatus}` + 哨兵错误 `ErrThrottled / ErrAuthFailed / ErrNotFound / ErrInvalidParameter`，含 `Unwrap` 链
- [ ] 3.3 编写 `client_test.go`：用 `httptest.NewServer` 覆盖 happy/限流/鉴权失败/byteplus root-level Error 四种响应形态

## 4. CP 子包：工作区 / 服务连接

- [ ] 4.1 实现 `cp/workspace.go`：`Create / List / Update / Delete` + `ListAll`（自动分页）
- [ ] 4.2 实现 `cp/serviceconn.go`：`Create / Get / List / Update / Delete`，请求体支持 github / gitlab / general-git / cr 几种 Type
- [ ] 4.3 编写 `cp/workspace_test.go` + `cp/serviceconn_test.go`：用 stub Client 校验请求体序列化与响应解码

## 5. CP 子包：流水线 / 运行 / 日志

- [ ] 5.1 实现 `cp/pipeline.go`：`CreatePipeline / ListPipelines / DeletePipeline`，Parameters/Caches 字段白名单 sanitizer，注释标注「unstable contract」
- [ ] 5.2 实现 `cp/run.go`：`RunPipeline / ListPipelineRuns / ListTaskRuns / CancelPipelineRun`
- [ ] 5.3 实现 `cp/log.go`：`GetTaskRunLog`（cursor 分页迭代器）+ `GetTaskRunLogDownloadURI`
- [ ] 5.4 编写对应 `*_test.go`：覆盖请求字段、响应解码、白名单过滤、cursor 终止条件

## 6. Mock 模式

- [ ] 6.1 实现 `mock.go`：`mockClient` 与 `Client` 同接口，进程内 map 维护 workspace/serviceConn/pipeline/run 状态
- [ ] 6.2 RunPipeline mock 默认 30s 后随机 Succeeded/Failed，可通过 `VOLC_MOCK_RUN_DURATION` / `VOLC_MOCK_FAIL_RATE` 调整
- [ ] 6.3 `volc.NewClient(target)` 在 `VOLC_MOCK=1` 时返回 mockClient；编写 `mock_test.go` 覆盖 happy 生命周期与「PipelineNotFound」分支

## 7. cpcheck CLI

- [ ] 7.1 实现 `cmd/cpcheck/main.go`：`--target` 参数 + 调用 `ListWorkspaces(PageSize=1)`，输出 `RequestId / Count / FirstName`
- [ ] 7.2 缺失凭证时退出码 2，调用失败退出码 1，成功 0
- [ ] 7.3 在仓库根 `Makefile` 增补 `make ping-pre` / `make ping-prod-cn` 目标，`go run ./cmd/cpcheck --target=...`

## 8. 集成测试与文档

- [ ] 8.1 新增 `server/internal/volc/cp/integration_test.go`，build tag `integration`：跑 ping → 创建临时 workspace → 删除（仅在 `VOLC_INTEGRATION=1` 时执行）
- [ ] 8.2 在 `server/internal/volc/README.md`（新增）中标注 CreatePipeline / CreateServiceConnection 为 unstable contract，列出 target 表与凭证表
- [ ] 8.3 运行 `go test ./server/internal/volc/...` 全绿；`VOLC_MOCK=1 go run ./server/cmd/cpcheck --target=prod-cn` 输出 mock 数据成功

## 9. 验证与归档准备

- [ ] 9.1 `openspec validate add-volc-cp-client --strict` 通过
- [ ] 9.2 在 `PLAN.md` 末尾追加链接指向本 change，便于后续 change 引用 `volc-cp-client` 能力
- [ ] 9.3 准备好 commit 草稿（`docs(openspec): add add-volc-cp-client proposal` 与 `feat(volc): introduce CP OpenAPI client` 两次提交分离）
