# PLAN：统一 HTTP 响应格式（server + webapp + Apifox）

> 对应 SPEC：[2026-06-13-01-unified-response-format.md](../issues/2026-06-13-unified-response-format.md)
> 分支：`feat-unified-response`（off `master`，小步提交）
> 三处同步：① server（Go）② webapp（TS）③ Apifox 项目 3807162（RSS-ZERO）
> 已决：迁移全部 JSON(`/api/v1`) 端点；采用集中式 error handler；`common.ApiResp` 全量替换为 `httputil.Resp` 并删除。

---

## 目标格式（重申）

- 成功：`{"message": "...", "data": <T>}`（`httputil.Resp[T]`，**无 `omitempty`**）；无数据 `{"message": "..."}`（`EmptyResp`）。
- 错误：`{"message": "..."}` + 正确状态码，由集中式 `HTTPErrorHandler` 统一产出。
- 边界：`/rss/*`(12 个 feed) 成功体保持 atom XML,**不套信封**;`/api/v1` 全部套。

---

## 现状盘点（实现时的依据）

- server:219 处 `c.JSON`(约 56 成功 / 165 错误),散落 14 个 controller 的 31 个文件;`common.ApiResp` 被引用 123 处;`archive.ErrResponse` 在 archive 包内重复定义并被多处使用。
- webapp:`src/api/client.ts` 的 `request<T>` 只 `return res.data`(裸 body);`fetchUserInfo`/`fetchZvideos` 手动再 `.data`;`BookmarkListResponse`/`AllTagsResponse`/`NewBookmarkResponse` 类型各多包一层。
- Apifox(项目 3807162):66 个接口，3 个共享 schema(`Topic`=144682195、`Cookie`、`Archive Fetch Base`),**无响应信封 schema**;每个接口响应**内联**定义;`Topic` 经 `$ref` 复用。`/api/v1` JSON 接口约 53 个，`/rss/*` 12 个，`/data` 1 个 (知乎加密内部端点)。

---

## 阶段 0 — 分支与基础设施

1. `git checkout -b feat-unified-response`。
2. 新建 `pkg/httputil/resp.go`：
   - `Resp[T any]{Message string`json:"message"`; Data T`json:"data"`}`（**无 omitempty**）
   - `EmptyResp{Message string`json:"message"`}`
   - `NewResp[T](msg, data) Resp[T]`、`NewMessage(msg) EmptyResp`
3. 新建 `pkg/httputil/error.go`：
   - `ResponseError{Code int; Message string}` + `Error()` + `NewHTTPError(code, msg)`
   - `NewHTTPErrorHandler(...) echo.HTTPErrorHandler`（**echo v4 签名** `func(err error, c echo.Context)`）：
     - `c.Response().Committed` 提前返回；
     - `errors.As(*ResponseError)` → `c.JSON(re.Code, NewMessage(re.Message))`；
     - `errors.As(*echo.HTTPError)` → `c.JSON(he.Code, NewMessage(http.StatusText(he.Code)))`（兜底 404/405/绑定错误）；
     - 否则 → `c.JSON(500, NewMessage(http.StatusText(500)))`；
     - 统一用请求级 logger 记 error（取法见风险 1）。
4. `cmd/server/echo.go`：`e.HTTPErrorHandler = httputil.NewHTTPErrorHandler(...)`。
5. `go build ./... && go vet ./...` 通过。
6. **提交**：`feat(httputil): add unified response envelope and centralized error handler`。

---

## 阶段 1 — 迁移 controller（小步，分批提交）

每个 handler 两类改动：

- 成功：`return c.JSON(http.StatusOK, httputil.NewResp("success", data))`（或 `NewMessage(...)`）。
- 错误：把 `return c.JSON(4xx/5xx, ErrResponse{Message: m})` → `return httputil.NewHTTPError(4xx/5xx, m)`。

每批结束 `go build`/`go vet`，单独提交。

- **批 A（webapp 直连，先做、最关键）**：`archive/`（archive、random、similarity、statistics、zvideo、bookmark、select）、`user/`。
  - 注意：archive/random/statistics/zvideo 当前是**裸返**，迁移后套信封——这是 webapp 必须同步的根因端点。
  - 提交：`refactor(archive,user): adopt unified response envelope`。
- **批 B（运维端点）**：`zhihu/`、`github/`、`xiaobot/`、`zsxq/`、`job/`、`cookie/`、`parse/`、`migrate/`、`macked/`、`rsshub/`、`endoflife/`。
  - RSS handler(`*/rss.go`、`c.String` 返回 XML)：**成功体不动**;仅把错误 `c.String(4xx,...)`/`c.JSON(4xx,...)` 改为 `return httputil.NewHTTPError(...)`(可选，统一错误)。
  - 按 controller 分多次提交，如 `refactor(zhihu): adopt unified response envelope`。

---

## 阶段 2 — 删除遗留类型

1. 删除 `internal/controller/archive/models.go` 的 `ErrResponse`；改用 `NewHTTPError`。
2. 全量替换 `common.ApiResp`/`WrapResp`/`WrapRespWithData`（123 处）为 `httputil.Resp`/`NewResp`/`NewMessage`，删除 `internal/controller/common/http_models.go` 中的对应类型。
3. `grep -rn "ApiResp\|ErrResponse\|c.JSON(http.Status\(Bad\|Internal\|Forbidden\)" internal/ pkg/` 应无残留错误返回。
4. `go build`/`go vet` 通过。**提交**：`refactor: remove legacy ApiResp/ErrResponse in favor of httputil`。

---

## 阶段 3 — webapp 配合（`../webapp`，独立分支同名）

1. `src/api/client.ts`：`request<T>` 改为拆一层信封：
   ```ts
   interface Envelope<T> { message: string; data: T }
   async function request<T>(call: () => Promise<AxiosResponse<Envelope<T>>>): Promise<T> {
     const res = await call();
     return res.data.data;
   }
   ```
2. 拍平各 fetch：`fetchUserInfo`/`fetchZvideos` 去掉手动 `.data`；删除 `BookmarkListResponse`/`AllTagsResponse`/`NewBookmarkResponse` 的外层包装类型，返回内层形状；archive/random/similarity/statistics 签名不变（靠统一拆信封保持正确）。
3. 错误拦截器不动（错误体仍 `{message}`）。
4. `bun run build`(tsc) 通过。**提交**(webapp 仓):`refactor(api): unwrap unified response envelope`。

---

## 阶段 4 — Apifox 同步（项目 3807162，用 MCP，写前先给 diff）

**手法**：逐接口 `readEntityDetails`(endpoint) 取当前 `definition` → 在其基础上改 → `updateHttpEndpoint` 提交。**不**用 `importData` 整体覆盖（会冲掉标题/描述/示例/状态/`x-apifox-*` 等元数据）。

每个 `/api/v1` JSON 接口的改动：

1. **成功响应**：把现有 `responses.<2xx>.content.application/json.schema`（记为 `S`）替换为：
   ```jsonc
   {
     "type": "object",
     "properties": { "message": { "type": "string" }, "data": S },
     "required": ["message", "data"],
     "x-apifox-orders": ["message", "data"]
   }
   ```
   - `data` 内保留对 `#/components/schemas/144682195`(Topic) 等的 `$ref`，复用不变。
   - 无数据的接口 (纯消息):`data` 省略，仅 `{message}`。
2. **错误响应**：为 400/403/500(按该接口实际)补/改 schema 为 `{ "type":"object", "properties": { "message": {"type":"string"} }, "required":["message"] }`。
   - 为减少重复，**可选**先用 `importData`(`schemaOverwriteMode: merge`) 注入一个共享错误 schema `Message`，错误响应统一 `$ref` 之；若 MCP 无独立 createSchema 工具，则内联（当前 18 工具无 schema CRUD，倾向内联）。
3. 范围：仅 `/api/v1` 的 ~53 个接口;**跳过** `/rss/*`(XML) 与按需评估 `/data`。

**执行约束**：

- 写 Apifox 是对线上外部服务的变更——**每批 update 前把"改前→改后 schema diff"列给作者确认**，确认后再调用 `updateHttpEndpoint`。
- 分批：先批 A 的 webapp 直连接口（archive/bookmark/tag/user/statistics/zvideo/random/similarity），核对无误后再批 B 运维接口。
- 可在 `.apifox/3807162_RSS-ZERO.settings.json` 落结构缓存 (MCP 建议，加 `fetchedAt`);把 `.apifox/` 加进 `.gitignore`。

---

## 阶段 5 — 验证与收尾

1. server:`go build ./... && go vet ./...`;`curl` 抽样 `archive`/`bookmark`/`tag`/`user`/`statistics`/`zvideo`/`random`/`similarity` 成功体为 `{message,data}`、错误体为 `{message}`+正确码;`/rss/*` 成功体仍 atom XML。
2. webapp:`bun run build`;八个页面端到端 (archive/random/bookmark/statistics/zvideo/similar drawer/tag 过滤/user),收藏乐观更新正常。
3. Apifox:抽样接口在界面查看响应 schema 已为信封;运行已有正向/负向测试用例 (若依赖响应结构需同步用例——见风险 4)。
4. 文档：更新 `docs/PROGRESS.md`;把实施中的经验整理进 `docs/lessons/2026-06-13-unified-response-format.md`。
5. 前后端**同分支/同次发布**切换 (破坏性，无双挂兼容期);Review 通过后 squash merge，删分支。

---

## 风险/待决

1. **logger 取法**：handler 内经 `InjectLogger` 中间件把 logger 放进 context；`NewHTTPErrorHandler` 内用 `c.Request().Context()` 取出记 error。实现阶段确认中间件存的 key/取法,取不到则降级到全局 logger。
2. **echo 自身错误**:绑定失败/404/405 会进 `HTTPErrorHandler`,需 `errors.As(*echo.HTTPError)` 兜底成 `{message}`,否则漏网为 echo 默认响应。
3. **`/data` 与 `/api/v1/health`**:`/data` 是知乎加密内部端点，确认调用方 (加密服务) 是否容忍信封;`/health` 已确认无探活依赖，套信封安全。
4. **Apifox 测试用例**:项目有 5 个用例分类 (正向/负向/边界/安全/其他)。若用例断言了响应结构，改信封后需同步用例 (`listTestCases`/`updateTestCase`)——实现阶段先 `readEntityDetails ... with=testCase` 评估影响面。
5. **Apifox 写入幂等**:`updateHttpEndpoint` 以完整 definition 提交，务必"读 - 改 - 写",避免漏字段导致覆盖丢失;每批先 diff 确认。

---

## 提交顺序小结

1. `feat(httputil)`：基础设施 + 挂 handler（阶段 0）
2. `refactor(archive,user)`：批 A（阶段 1）
3. `refactor(<controller>)`×N：批 B（阶段 1）
4. `refactor`: 删 ApiResp/ErrResponse（阶段 2）
5. webapp 仓：`refactor(api)`（阶段 3）
6. Apifox：MCP 写入（阶段 4，非 git，分批 diff 确认）
7. docs：PROGRESS + LESSON（阶段 5）
