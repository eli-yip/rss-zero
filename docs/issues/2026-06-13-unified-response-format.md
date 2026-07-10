# SPEC：统一 HTTP 响应格式（对齐 maestro-engine）

> 状态：**待定稿 / 待实现**
> 范围（server）：新增 `pkg/httputil/**`；改 `cmd/server/echo.go`（挂 error handler）；迁移 `internal/controller/**` 所有 **JSON**（`/api/v1`）handler 的成功与错误返回。
> 范围（webapp）：`src/api/client.ts` 的 `request<T>` 统一拆信封，拍平各 fetch 函数的返回类型。
> 参考实现：`linker-bot/maestro/maestro-engine/pkg/httputil`（`resp.go` + `error.go`）。
> 已决（与作者确认）：① 迁移**全部** JSON API 端点（非仅 webapp 用的 8 个）；② **一并**采用集中式 error handler。

---

## 背景与核心判断

当前 rss-zero 的 JSON 响应是**三套并存**：

| 形态                                                    | 出处                                                                     | 端点举例                               |
| ------------------------------------------------------- | ------------------------------------------------------------------------ | -------------------------------------- |
| `common.ApiResp[T]{message,data}`（**带 `omitempty`**） | `internal/controller/common/http_models.go`                              | bookmark / tag / user                  |
| 裸 payload（无信封）                                    | `archive/models.go` 的 `ArchiveResponse`/`ResponseBase`/`ZvideoResponse` | archive / random / similarity / zvideo |
| 裸 map                                                  | `archive/statistics.go`                                                  | statistics                             |

错误处理则**完全手写**：每个 handler 自己 `c.JSON(http.StatusXxx, ErrResponse{...})`，`ErrResponse` 在 `archive` 包内重复定义；**没有任何集中式 `HTTPErrorHandler`**。全仓 219 处 `c.JSON`（约 56 成功 / 165 错误）散落在 14 个 controller。

后果：

- 前端 `request<T>`（`client.ts:35`）对不同端点要区别对待——`fetchUserInfo`/`fetchZvideos` 手动 `.data` 解包，archive 系列又不解包；类型层 `BookmarkListResponse`/`AllTagsResponse`/`NewBookmarkResponse` 各自多包一层。
- `omitempty` 使空 `data` 直接消失，client 的 `.data` 取值不可靠。
- 错误形状靠各 handler 自觉，无单一出口、无统一日志。

**判断**：maestro-engine 已有一套经过验证的极简方案，且是同一作者的姊妹项目（共享心智模型、未来可共享代码）。照搬之，让"成功永远 `{message,data}`、错误永远 `{message}`、错误集中处理"。

---

## 目标格式（照搬 maestro，适配 echo v4）

### `pkg/httputil/resp.go`

```go
package httputil

// 注意：不带 omitempty —— 成功响应形状恒定，便于前端无条件解包。
type Resp[T any] struct {
	Message string `json:"message"`
	Data    T      `json:"data"`
}

type EmptyResp struct {
	Message string `json:"message"`
}

func NewResp[T any](message string, data T) Resp[T] { return Resp[T]{Message: message, Data: data} }
func NewMessage(message string) EmptyResp           { return EmptyResp{Message: message} }
```

### `pkg/httputil/error.go`

```go
package httputil

type ResponseError struct {
	Code    int
	Message string
}

func (e *ResponseError) Error() string { return e.Message }

func NewHTTPError(code int, message string) *ResponseError {
	return &ResponseError{Code: code, Message: message}
}
```

集中式错误处理器（**echo v4 签名** `func(err error, c echo.Context)`，区别于 maestro v5 的 `func(*echo.Context, error)`）：

```go
func NewHTTPErrorHandler(logger ...) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}
		// 统一记日志（用项目现有的 request-scoped logger）
		code := http.StatusInternalServerError
		var re *ResponseError
		if errors.As(err, &re) {
			code = re.Code
			_ = c.JSON(code, NewMessage(re.Message))
			return
		}
		// 兼容 echo 自身抛出的 *echo.HTTPError（404/405 等）
		var he *echo.HTTPError
		if errors.As(err, &he) {
			code = he.Code
			_ = c.JSON(code, NewMessage(http.StatusText(code)))
			return
		}
		_ = c.JSON(code, NewMessage(http.StatusText(code)))
	}
}
```

在 `cmd/server/echo.go` 里 `e.HTTPErrorHandler = httputil.NewHTTPErrorHandler(...)`。

### Handler 约定（迁移后）

- 成功：`return c.JSON(http.StatusOK, httputil.NewResp("success", data))`；无数据时 `httputil.NewMessage("...")`。
- 错误：`return httputil.NewHTTPError(code, msg)` —— **不再**在 handler 内 `c.JSON(4xx/5xx, ...)`。
- 删除 `archive` 包内重复的 `ErrResponse`；现有 `common.ApiResp` 由 `httputil.Resp` 取代（或将 `common` 改为转发 `httputil`，二选一，见 PLAN）。

---

## 边界（必须遵守）

1. **RSS feed 端点（`/rss/*`，12 个）不套 JSON 信封。** 它们按 `Content-Type` 契约返回 atom XML（`c.String`/XML），RSS 阅读器依赖原始 XML。其**错误**路径可改为 `return NewHTTPError(...)`（让阅读器拿到 JSON 错误无伤大雅），但**成功体保持 XML 原样**。"全部端点"在实现上 = 全部 **JSON**（`/api/v1`）端点。
2. **`/api/v1/health`** 也套信封（`NewResp("ok", versionInfo)`），统一即统一；若外部探活依赖裸形状，迁移时单列确认。
3. 本次**只改响应格式**，不动路径、不动 HTTP 方法、不动 `content_type` 字符串/整数枚举混用等其它 SPEC 议题——那些留给后续。

---

## webapp 配合（`src/api/client.ts`）

`request<T>` 改为统一拆一层信封：

```ts
interface Envelope<T> { message: string; data: T }
async function request<T>(call: () => Promise<AxiosResponse<Envelope<T>>>): Promise<T> {
  const res = await call();
  return res.data.data; // axios body(.data) → 信封 (.data)
}
```

随之：

- `fetchUserInfo` / `fetchZvideos`：去掉手动 `.data` / 中间类型，直接 `request<{username,nickname}>` / `request<{zvideos:Zvideo[]}>`。
- `fetchBookmarkList` / `fetchAllTags` / `addBookmark` / `removeBookmark` / `updateBookmark`：删掉响应类型里多包的那层（`BookmarkListResponse`/`AllTagsResponse`/`NewBookmarkResponse` 拍平成内层 `data` 的形状）。
- archive / random / similarity / statistics：fetch 函数签名不变（仍返回内层形状），但因服务端现在套了信封、`request` 统一拆信封而保持正确。
- 错误拦截器（`client.ts:16-29`）无需改：错误体仍是 `{message}`，`error.response.data.message` 照常工作。

---

## 验收

- server：`go build ./...` + `go vet ./...` 通过；全仓无裸 `c.JSON(4xx/5xx, ...)` 错误返回（错误统一走 `NewHTTPError`）；无 `archive.ErrResponse` 残留；`/api/v1/*` 成功体一律 `{message,data}`。
- 契约：抽样 `curl` 验证 archive/bookmark/tag/user/statistics/zvideo/random/similarity 成功体均为 `{message,data:...}`，错误体为 `{message:...}` + 正确状态码；`/rss/*` 成功体仍为 atom XML。
- webapp：`bun run build`（tsc）通过；八个页面（archive/random/bookmark/statistics/zvideo/similar drawer/tag 过滤/user）端到端正常，收藏乐观更新正常。
- 前后端在**同一分支/同一次发布**内切换（破坏性变更，单部署、作者同时掌控两端，无需双挂兼容期）。

---

## 实施顺序（细节见 PLAN）

1. 新建 `pkg/httputil`（resp + error + handler），挂到 `echo.go`，`go build` 通过。
2. 迁移 controller（按从前端依赖到运维分批，小步提交，每步 `go build`/`go vet`）：
   - 批 A（webapp 直连）：`archive`（archive/random/similarity/statistics/zvideo/bookmark/select）、`user`。
   - 批 B（运维）：`zhihu`、`github`、`xiaobot`、`zsxq`、`job`、`cookie`、`parse`、`migrate`、`macked`、`rsshub`、`endoflife`。
   - 收尾：删除 `archive.ErrResponse` 与 `common.ApiResp`（或将 `common` 转发到 `httputil`）。
3. webapp 改 `request<T>` 并拍平类型，`bun run build`。
4. 抽样 `curl` + 页面回归；更新 PROGRESS / LESSON。

---

## 风险/待决

1. **logger 接入**：maestro 的 handler 持有 `mlog.Logger`。rss-zero 的请求级 logger 经 `InjectLogger` 中间件注入 context（`myMiddleware`）。handler 内用 `c.Request().Context()` 取 logger 记错误即可——实现时确认取法。
2. **echo 自身错误**（404/405/body 解析失败）：未被 handler 捕获时会进 `HTTPErrorHandler`，需 `errors.As(*echo.HTTPError)` 兜底成 `{message}`，避免漏网返回 echo 默认 HTML/JSON。
3. **`common.ApiResp` 的去留**：当前 123 处引用。两种收尾——(a) 直接全量替换为 `httputil.Resp` 并删 `common` 类型；(b) 让 `common` 包薄转发到 `httputil` 以减少 diff。建议 (a)，彻底单一来源；diff 大但机械。PLAN 阶段定。
4. **health 探活**：若部署侧（compose healthcheck / 反代）对 `/health` 形状有硬依赖，套信封前需确认。
