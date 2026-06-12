# PLAN：项一 — 请求重试循环收敛

> 对应 SPEC：[2026-06-12-01-zsxq-maintainability-refactor.md](../specs/2026-06-12-01-zsxq-maintainability-refactor.md) 重构项一
> 分支：`feat-zsxq-request-retry`
> 风险：中（改动核心爬取链路；request 包当前**无任何测试**，需先补回归测试再重构）

## 目标

把 `Limit`/`LimitRaw`/`LimitStream` 的雷同重试循环抽为单一私有方法 `doWithRetry`，三个公开方法降为薄包装。删除无调用者的 `NoLimit`。对外保留的三方法签名、错误类型、行为不变，并修掉循环内 `defer` 的 body 泄漏。

## 前置事实（已核实）

- 保留三方法对外签名见 `Requester` 接口（request.go:39–50），**不能变**。
- zsxq 侧调用点：
  - `Limit`：cmd/server/echo.go:328、parse/topic.go:141、crawl/group.go:44
  - `LimitRaw`：parse/talk.go:96
  - `LimitStream`：parse/talk.go:62、image.go:39、q&a.go:46
  - `NoLimit`：**无调用者**（仅接口声明）→ 本 PR 直接删除（接口 + 实现）。
- 删 `NoLimit` 后三方法都走 limiter，故 `doWithRetry` **不需要 `useLimiter` 参数**。
- `r.limiter` 来自全局 `TokenPool`，每 ~30s 才发一个 token：**测试不能走真 limiter**，否则重试路径会阻塞 30s/次。测试用白盒方式直接构造 `RequestService`，注入一个不阻塞的 limiter channel。

## 实现设计（来自 SPEC 定稿）

核心：
```go
// doWithRetry runs a limited+retried GET that must yield HTTP 200.
// On each 200 it calls validate(resp):
//   done=true  -> return this resp (caller owns Body)
//   done=false -> retry (validate must Close Body if it read it)
//   err!=nil   -> fatal, return immediately (e.g. ErrInvalidCookie)
// Non-200 and transport errors are retried; Body closed before retry.
// After maxRetry, returns ErrMaxRetry (or last err).
func (r *RequestService) doWithRetry(
    ctx context.Context, u string,
    validate func(resp *http.Response) (done bool, err error),
    logger *zap.Logger,
) (*http.Response, error)
```

要点：
- taskID 生成、per-attempt 子 logger、`for i < maxRetry`、终态 `if err==nil { err = ErrMaxRetry }` 全部进核心，唯一真相。
- 非 200：`resp.Body.Close()` 后 `continue`（修掉现在的 defer 泄漏）。
- 200：调 `validate`；`err!=nil` 直接返回；`done` 返回 resp；否则 continue。
- 核心**不关** 200 且交给 validate 的 body——读字节的包装在 validate 内即时 Close，stream 包装故意不 Close。

薄包装：
- `Limit`：闭包变量 `body`；validate 读 body→Close→解析 `apiResp.Succeeded`；成功存 body 返回 done。失败解析 `badAPIResp.Code`：401→`return false, ErrInvalidCookie`；1059→`Sleep(60s); return false,nil`；1050（保留现有 dataSystemResp 日志）及 default→`return false,nil`。最后 `return body, nil`。
- `LimitRaw`：闭包 `body`；validate 读 body→Close→存 body→done。
- `LimitStream`：`validate = func(*http.Response)(bool,error){ return true,nil }`，直接返回核心结果。

删除：`NoLimit` 方法实现 + `Requester` 接口中的声明。

保持不变：`ErrInvalidCookie`/`ErrMaxRetry`、日志文案尽量沿用、`setReq`、`apiResp`/`badAPIResp`/`dataSystemResp` 类型。

## 步骤

### 1. 建分支
```
git checkout -b feat-zsxq-request-retry
```

### 2. 先补回归测试（重构前锁定行为）
新增 `pkg/routers/zsxq/request/request_test.go`，白盒（package request），用 `httptest.Server` + 注入非阻塞 limiter：
- 构造 helper：`newTestService(t, handler)` → 直接 `&RequestService{client: srv.Client()/&http.Client{}, maxRetry: N, logger: zap.NewNop(), limiter: ready}`，其中 `ready` 为已预填满的 buffered channel 或一个持续供给的 goroutine。
- 用例：
  1. `Limit` 200 `{"succeeded":true,...}` → 返回原始 body。
  2. `Limit` 200 `{"succeeded":false,"code":401}` → `ErrInvalidCookie`，不重试。
  3. `Limit` 200 `{"succeeded":false,"code":1050}` → 重试至 `ErrMaxRetry`（1059 因 `Sleep(60s)` 不直接测，或将 sleep 抽为可注入字段——本轮**不引入**，仅测 1050/default 重试路径）。
  4. `Limit` 非 200（500）→ 重试至 `ErrMaxRetry`。
  5. `LimitRaw` 200 → 返回 body 原文（不校验 JSON）。
  6. `LimitStream` 200 → 返回 `*http.Response`，调用方能从 `resp.Body` 读到完整内容（验证 body 未被提前关闭）。
  7. `LimitStream` 非 200 → 重试至 `ErrMaxRetry`。
- 先在**重构前**跑通这些测试（对照现有实现），确保它们刻画的是当前行为。

> 注：1059 的 `time.Sleep(60s)` 不在单测覆盖（会拖慢）。如需覆盖，可在后续把 sleep 提为 `RequestService` 可注入的字段；本 PR 不动，避免扩大范围。

### 3. 实现 `doWithRetry` 并改写四方法
- 加入核心方法，按上面设计改写四个公开方法为薄包装。
- 删除四处重复循环。

### 4. 跑测试，确认行为不变
```
go test ./pkg/routers/zsxq/request/...
go build ./...
go vet ./pkg/routers/zsxq/...
```

### 5. 真实链路冒烟（可选但建议）
- `cmd/server/echo.go` 的 zsxq cookie 测试端点会用 `Limit` 打 `config.C.TestURL.Zsxq`，本地起服务点一次确认 200 路径正常。

## 验收

- `Requester` 四方法签名与错误类型不变；调用点零改动即可编译。
- 重试逻辑仅 `doWithRetry` 一处；request.go 约 200 行 → 核心 ~35 行 + 四薄包装。
- 所有分支 body close 统一、无泄漏（循环内 defer 消除）。
- 新增 request_test.go 覆盖 6–7 个行为用例并通过。
- `go build ./...` / `go vet` 通过。

## 提交与合并

- Conventional Commit（英文），如：
  `test(zsxq): add request retry regression tests`
  `refactor(zsxq): consolidate request retry into doWithRetry`
- 完成后请作者 review，批准后 squash merge 进 `master` 并删分支。
- 更新 `docs/PROGRESS.md`。
