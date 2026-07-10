# LESSON：统一 HTTP 响应格式（server + webapp + Apifox）

> 对应 SPEC/PLAN：[2026-06-13-01-unified-response-format](../issues/2026-06-13-unified-response-format.md)
> 结果：server `feat-unified-response`（6 commits）+ webapp 同名分支（1 commit）+ Apifox 项目 3807162 全量信封化。`go build`/`go vet`/`tsc` 均绿。待 review/合并。

## 成果

- **格式**（照搬 maestro-engine）：成功 `{message, data}`（`httputil.Resp[T]`，无 `omitempty`），无数据 `{message}`（`EmptyResp`）；错误 `{message}` + 状态码，由集中式 `HTTPErrorHandler` 统一产出。
- **server**：`pkg/httputil` 单一来源；handler 成功用 `NewResp`/`NewMessage`，错误一律 `return httputil.NewHTTPError(code,msg)`；删除 `common.ApiResp`/`WrapResp`/`EmptyData`、`archive.ErrResponse`、job 死类型。`/rss/*` XML 成功体保持不变（按内容类型契约）。`/health` 也信封化。
- **webapp**：`request<T>` 改为 `res.data.data` 统一拆一层信封；user/tags/bookmark-list/bookmark-mutations 的响应类型拍平（删掉外层 wrapper）。错误拦截器无需改（错误体仍 `{message}`）。
- **Apifox**：53 个 `/api/v1` JSON 接口响应 schema 全部信封化（42 改 + 10 本已信封），错误补 `{message}`。

## 关键经验

1. **echo v4 的 HTTPErrorHandler 签名是 `func(err error, c echo.Context)`**（maestro 用的是 v5 的 `func(*echo.Context, error)`）。handler 内取请求级 logger：`c.Get("logger").(*zap.Logger)`，取不到降级到注入的 base logger。必须 `errors.As(*echo.HTTPError)` 兜底，否则 echo 自身的 404/405/绑定错误会漏成默认响应。

2. **机械迁移用 perl 批量替换最省力，但要避开两个坑**：
   - **zsh 默认不做无引号变量分词**：`perl -pi -e '...' $FILES` 会把整串当成一个文件名。用 `find … -print0 | xargs -0 perl …`，别用 `$FILES`。
   - **正则用贪婪 `(.+)` 而非非贪婪**：`common.WrapResp(err.Error())` 内部含括号，非贪婪 `(.+?)\)\)` 会在 `err.Error(` 处提前收尾；贪婪 `(.+)\)\)` 回溯到最后两个 `)` 才正确。

3. **`httputil` 与 stdlib `net/http/httputil` 同名**：不能让 goimports 自动补我们的 `pkg/httputil`（它可能选 stdlib）。做法是先显式插入 import 再跑 goimports（goimports 只删未用、不动已用且存在的 import）。

4. **成功响应分两类，不能一刀切包 `{message,data}`**：纯动作端点（refmt/migrate/delete/activate…）服务端返回 `NewMessage` → `{message}`；有数据的返回 `NewResp` → `{message,data}`。分类的唯一可靠来源是**服务端代码**（grep `NewResp` vs `NewMessage`），不能从旧 Apifox schema 反推。

5. **Apifox 新版 MCP（Streamable HTTP，`https://api.apifox.com/mcp` + `X-Apifox-Api-Version: 2025-09-01`）支持写**（`createHttpEndpoint`/`updateHttpEndpoint`/`importData`），旧版 stdio `apifox-mcp-server` 只读。桌面 App 启动拿不到 shell/direnv 环境，`${VAR}` 展开为空 → 用 `launchctl setenv` 把 token 注入 launchd 会话，GUI App 才继承。
   - `updateHttpEndpoint` **只传 `responses` 字段即可**，其余（summary/requestBody/参数/`x-apifox-*`）原样保留——比 `importData` 整体覆盖安全。共享 schema 的 `$ref`（如 Topic=144682195）保留即可，不必内联。

6. **批量写 Apifox 前先验证一个**：改完一个 endpoint 立即 `readEntityDetails` 回读，确认元数据没被覆盖，再放心批量。

## 遗留 / 后续

- **cookie 路由漂移（已清理）**：Apifox 原是旧的 per-platform 路由（`/cookie/{zsxq,zhihu,github,xiaobot}`，6 个），服务端 unified-cookie 改造后已是通用 `POST/GET /api/v1/cookie`。已删除 6 个旧接口、在 Cookie 目录新建通用 `POST /api/v1/cookie`（body `{cookies:[…]}` → `{message,data:{results:[…]}}`）与 `GET /api/v1/cookie`（→ `{message,data:{cookies:[…]}}`）与服务端对齐。
- **本次只改响应格式**，未动其它 API 设计议题（POST-查询改 GET、content_type 字符串/整数枚举统一、bookmark 子资源化等）——见首性原理讨论，留作后续 SPEC。
- **运行时验证**：build/vet/tsc 已过，但未起服务做 curl/页面回归（需 DB/redis/cookie）。合并/部署前应抽样验证响应形状与八个前端页面。
