# PLAN: tombkeeper 微博 RSS 订阅源

- 日期：2026-06-18
- SPEC：[`docs/specs/2026-06-18-01-tombkeeper-rss.md`](../specs/2026-06-18-01-tombkeeper-rss.md)
- 解析 SSOT：[`pkg/routers/tombkeeper/example/README.md`](../../pkg/routers/tombkeeper/example/README.md)
- 分支：`feat-tombkeeper-rss`

把 SPEC 拆成可独立提交、可验证的步骤。包结构对齐 `macked`（扁平单包 `pkg/routers/tombkeeper`，
多文件），但功能更全（图片/DB/短链/转发）。解析规则细节一律以 SSOT 为准，本 PLAN 只列**步骤、
文件、接口签名、验证方式**。

## 集成锚点（来自代码扫描）

- **单源模板 `macked`**：`CrawlFunc(redisService redis.Redis, db DB, logger) func()`；
  `DBService struct{ *gorm.DB }` + `NewDBService`；`RSSRenderer{ RenderRSS, RenderEmptyRSS }`；
  cron 在 `cmd/server/cron.go` 的 `jobs []jobDefinition{name,schedule,fn}` + `AddJob`；
  controller 在 `internal/controller/macked/`（`controller.go`+`rss.go`）；路由 `/rss/macked`
  在 `cmd/server/echo.go`；redis 常量 `RssMackedPath`。
- **图片→OSS**：`file.File.SaveStream(path, io.ReadCloser, size)`、`AssetsDomain()`、`Exist()`；
  对象公链用 zsxq `Object.URI()` 风格（`StorageProvider[0] + "/" + PathEscape(objectKey)`）；
  下载带 `Referer: https://weibo.com`（仿 `weibo/request.GetPicStream`），重试参考 zhihu
  `NoLimitStream(ctx, client, url, maxRetry, logger)`。
- **Atom 渲染**：`github.com/gorilla/feeds`（`Feed`/`Item`/`ToAtom()`）；markdown→HTML 用
  `goldmark.New(WithExtensions(extension.GFM, extension.NewCJK(...)))` 的 `Convert([]byte,&buf)`；
  摘要 `render.ExtractExcerpt`；`render.BuildArchiveLink(serverURL, link)` =
  `serverURL + "/api/v1/archive/" + link`。markdown 拼装用 `internal/md`（`Quote`/`Image`/`Join`/`H*`）。
- **DB**：GORM tag + `TableName()`；在 `internal/migrate/db.go` 的 `AutoMigrate(...)` 注册；
  `NewPostgresDB` 启动建库；per-source `NewDBService(db)` 在 `cmd/server/echo.go` 注入。
- **image-seeker CDN**：主机 `ww/wx/tvax[1-4]` + 第三方代理（`image.baidu.com/search/down?url=`、
  `cdn.cdnjson.com/`、`i0.wp.com/`、`cdn.ipfsscan.io/weibo/`）；并发探测 `CheckCDNAvailability`
  （goroutine + `http.Head` + mutex 收集可用）。本源改为 **GET + Referer** 并发探测取首个 200。
- **archive 页**：`GET /api/v1/archive/:url` → `History`，按 URL 正则分派到各源 handler 产出
  `{title, markdown}` → `htmlRender.Render(title, md)`。tombkeeper 归档页加一个 weibo 正则分派即可。

## mid ↔ bid（已验证 7/7）

base62 字母表 **小写在前** `0-9a-zA-Z`；mid→bid 从右 7 位分组、内组左补 0 到 4 字符；
bid→mid 从右 4 字符分组、内组左补 0 到 7 位。实现 `MidToBid(string) string` /
`BidToMid(string) (string, error)`，单测用 SSOT §6 的真实 (mid,bid) 对。

## 步骤（每步一提交）

### 0. PLAN 提交（docs）

本 PLAN + PROGRESS 更新，作为 `docs(...)` 提交（SPEC 已先行提交）。

### 1. 基础工具：账号集合 + mid/bid（`pkg/routers/tombkeeper`）

- `uid.go`：硬编码账号集合 `{1401527553:"tombkeeper", 6827625527:"t0mbkeeper"}` +
  `IsTombkeeperUID(uid string) bool`（后续可抽接口）。
- `midbid.go`：`MidToBid` / `BidToMid` + base62 编解码。
- `midbid_test.go`：对 SSOT 真实 7 对双向断言；非法字符返回 error。
- **验证**：`go test ./pkg/routers/tombkeeper/ -run MidBid`。

### 2. flight 提取（`extract.go`）

- 类型 `RawPost`（镜像 flight JSON：id/bid/user_id/screen_name/text/pics/video_url/
  created_at/retweet_id/url_info 等）+ `URLInfoEntry{short_url,url_title,url_type,long_url,weibo_bid}`。
- `ExtractPosts(html []byte) ([]RawPost, error)`：① 正则取所有 `self.__next_f.push([1,"…"])`
  分片、JSON 解转义、按序拼接 → ② 花括号配平抽 `{"id":"<数字…>",…}` → ③ 处理 `$D`
  （`created_at` 剥前缀按 RFC3339）与 `$ref`（`url_info` 解引用为数组）。返回顺序保留。
- `extract_test.go`：用夹具对象合成 chunked flight 串（含 `$D`/`$ref`）→ 断言抽出对象数量与字段；
  另存一份真实页面到 `example/page_raw.html` 验证端到端（可选，若体积可接受）。
- **验证**：`go test -run Extract`。

### 3. 请求层（`request.go`）

- `RequestService`：`GetPage(page int) ([]byte, error)`（`https://tombkeeper.io/?page=N`，自定义 UA、
  限速、重试，无 cookie）；`GetDetail(id string) ([]byte, error)`（`/weibo/{id}`）；
  `GetPicStream(url string) (*http.Response, error)`（带 `Referer: https://weibo.com`）。
- **验证**：`go build`；GetPage 的轻量集成测试可跳过网络（仅签名/构造）。

### 4. 图片管线（`image.go`）

- `candidateURLs(picOrURL string) []string`：裸 id → `https://{ww|wx|tvax}{1-4}.sinaimg.cn/large/{picid}.jpg`
  - 第三方代理（baidu/cdnjson/i0.wp/ipfsscan）；完整 URL 直接返回该 URL。
- `downloadFirstAvailable(cands []string) (resp, usedURL, error)`：**并发** GET（带 Referer）探测，
  取首个 200 的响应体；全失败返回 error + 用于"保留原链"的首个 sinaimg 候选 URL。
- `saveImage(picID, postID, rawPicField)`：下载 → `file.SaveStream("tombkeeper/{picid}.jpg", body, size)`
  → `SaveObjectInfo(Object{status=0,object_key,url,storage_provider=[AssetsDomain]})`；全失败则
  `Object{status=1, object_key="", url=原链}`，返回"保留原链"标记。扩展名默认 jpg，按 `Content-Type` 取 gif。
- 幂等：`Exist`/DB 已有则跳过。
- `image_test.go`：候选 URL 生成断言（裸 id / 完整 URL 两形态）。
- **验证**：`go test -run Image`（仅候选生成，不打网络）。

### 5. 解析与 Markdown 渲染（`parse.go` + `render_markdown.go`）

- `render_markdown.go`：`escapeMarkdown(plain string) string`（转义 `*_[]()` 等）；
  `\n` → 换行；拼装 helper（正文 + 图片 `![](url)` + 视频链接 + 引用块）。
- `parse.go`：`ParsePost(raw RawPost, pageMap map[string]RawPost) (Post, []Object, error)`：
  正文 `text` 转义→markdown；`pics` 逐张过图片管线追加；`url_info` 视频项追加；**短链处理**
  （步骤 6）；**转发内联**（`retweet_id` 命中 `pageMap` → 引用块内联原文 + 图片，缺失回退一次
  `GetDetail`）；`title` = 正文前 10 字（去换行）。产出 `Post{id,bid,author_id,screen_name,
  created_at,title,text_markdown,video_url,retweet_id,raw}` + `[]Object`。
- `parse_test.go`：对 `example/*.json` 夹具断言（单图/多图/纯文本/转发/视频/微博正文链接）——
  图片下载用可注入的 stub（不打网络），断言 markdown 结构与 object 记录。
- **验证**：`go test -run Parse`。

### 6. url_info 短链处理（`shortlink.go`）

- `processShortLinks(text string, urlInfo []URLInfoEntry, deps) (markdown string, inlineQuotes []string, []Object)`：
  按 `short_url` 相等匹配；分类（tombkeeper 微博正文 / 普通）；微博正文 → 行内 `[微博正文N](rss_url)`
  （`rss_url = BuildArchiveLink(serverURL, "https://weibo.com/detail/{mid}")`）+ 文末引用块内联
  （流程：`IsTombkeeperUID` → `BidToMid` → 查 DB → `GetDetail` 抓站入库；**只 1 层**）；普通 →
  `[url_title](long_url)`。
- `shortlink_test.go`：`plain_text.json` 断言：3 个 `t.cn` 全部识别为微博正文、行内编号 1/2/3、
  文末 3 个引用块；DB/抓站用 stub。
- **验证**：`go test -run ShortLink`。

### 7. DB 模型与迁移（`db.go` + `internal/migrate/db.go`）

- `db.go`：`Post`、`Object` 模型（GORM tag + `TableName()` = `tombkeeper_post`/`tombkeeper_object`）；
  `DB` 接口（`DBPost`+`DBObject`）+ `DBService struct{ *gorm.DB }` + `NewDBService`；方法
  `SavePost`/`GetPost`/`GetLatestNPosts`/`PostExists`/`SaveObjectInfo`/`GetObjectInfo`。
- `internal/migrate/db.go`：import + `AutoMigrate(&tombkeeperDB.Post{}, &tombkeeperDB.Object{})`。
- **验证**：`go build`；本地起库 AutoMigrate 建表（或 `go vet`）。

### 8. RSS 渲染 + 抓取编排（`render_rss.go` + `crawl.go` + redis 常量）

- `internal/redis`：`RssTombkeeperPath = "tombkeeper_rss"`。
- `render_rss.go`：`RSSRenderer{ RenderRSS([]Post)(string,error); RenderEmptyRSS()(string,error) }`；
  Atom：item `Title=post.title`、`Content=goldmark(text_markdown)`、`Link=weibo.com/detail/{id}`、
  `Description=ExtractExcerpt`、`Created/Updated=created_at`。
- `crawl.go`：`CrawlFunc(redisService, db, file, logger) func()` → `Crawl(...)`：抓 2 页 →
  `ExtractPosts` → 建 `pageMap` → 逐条 `PostExists` 跳过 / `ParsePost` → `SavePost`+`SaveObjectInfo`
  → 渲染 RSS 写 redis（`RRUNDER`：取 DB 最新 N 条）。
- **验证**：`go build`；`crawl_test`（用 stub request/file/db 跑一遍 happy path）。

### 9. 接线（cron + controller + 路由）

- `internal/controller/tombkeeper/`：`controller.go`（deps: redis, db, logger）+ `rss.go`
  （读 redis 缓存，缺失则 DB 最新 N 条重渲染回填，仿 macked `rss.go`）。
- `cmd/server/cron.go`：jobs 加 `{name:"tombkeeper_crawl", schedule:"0 * * * *",
  fn: tombkeeper.CrawlFunc(redisService, tombkeeper.NewDBService(db), fileService, logger)}`。
- `cmd/server/echo.go`：构造 controller + 注册 `GET /rss/tombkeeper`（+ 可选 `/rss/tombkeeper/:feed`）。
- **验证**：`go build ./...`；本地起服务，`curl /rss/tombkeeper` 返回 Atom（debug 模式下 cron 不跑，
  可加一个手动触发或临时跑一次抓取）。

### 10. （后期）单条微博归档页 `rss_url`

- `internal/controller/archive/tombkeeper_archive.go`：`HandleTombkeeperPost`——URL 正则
  匹配 `weibo.com/detail/{id}`（及 `weibo.com/{uid}/{bid}`、`tombkeeper.io/weibo/{id}`），取 DB
  post → 渲染 markdown（**HTML 不显示标题**，只渲染正文）→ `{title:"", markdown}`。
- `cmd/server/echo.go` archive 分派加该正则。
- **验证**：`curl /api/v1/archive/https%3A%2F%2Fweibo.com%2Fdetail%2F{id}` 返回正文 HTML。

### 11. （后期）`title` 的 LLM 细化

- 在 `internal/ai` 既有 LLM 客户端基础上，把 `title` 从"前 10 字"升级为 LLM 概括；
  作为后续增量（可加一次性 backfill 迁移）。

### 12. 评审收口

- 跑 `just lint`（autocorrect/dprint/golangci-lint/go fix）修齐；
- 对全 diff 跑一轮对抗式代码评审（correctness / 边界 / SSOT 一致性 / 安全），按结果修复；
- 请作者 review → squash 合并 master。

## 测试与验证策略

- **纯函数**（midbid / 候选 URL / 短链分类 / markdown 转义）用夹具与真实样本做单测。
- **解析/渲染**对 `example/*.json` 断言；网络与 OSS 用可注入 stub，不打真实网络。
- **抓取/RSS** happy-path 用 stub 跑通；端到端在本地起服务用 `curl` 验证。
- 不跑全量历史回填；只增量 2 页。

## 风险与缓解

- flight 分片/`$ref` 格式变动 → 提取层容错（缺字段跳过、解析失败记日志不中断整页）。
- sinaimg 防盗链/CDN 全失败 → 保留原链、`status=1` 记录避免重复尝试。
- `bid→mid` 边界（短 mid/大小写）→ 单测覆盖；非法 bid 返回 error 走"普通链接"降级。
- 转发/短链内联只 1 层，避免递归与请求放大；抓站受请求层限速约束。
