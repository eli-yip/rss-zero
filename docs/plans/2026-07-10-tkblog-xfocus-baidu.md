---
title: "tombkeeper 博文（xfocus / baidu）解析落库与归档"
issue: docs/issues/2026-07-10-tkblog-xfocus-baidu.md
status: draft
areas: [tombkeeper, controller, migrate, db]
updated: "2026-07-10"
---

# PLAN: tombkeeper xfocus / baidu 博文

- 日期：2026-07-10 · SPEC：[issues/2026-07-10-tkblog-xfocus-baidu.md](../issues/2026-07-10-tkblog-xfocus-baidu.md)
- 分支：`feat-tkblog`

## 目标

把 xfocus / baidu 两批 tk 博客文章解析落库 + 提供单篇归档 HTML（见 issue）。两者页面模板相同、
比微博简单（纯文本、无转发/图片/嵌入），因此**新建独立包 `pkg/routers/tkblog`**，一套按
`category` 参数化的逻辑覆盖两源，复用现有共享层（archive 分派、`render` HTML、pseudo-job 套路）。

## 关键决策

1. **独立包 `pkg/routers/tkblog`，不塞进 `tombkeeper`。**
   微博包深度耦合 weibo 语义（mid/bid、转发、图片、url_info），博文的数据模型/表结构/链接语义
   完全不同（string id、category、wayback url、无媒体）。混入会污染已测的微博路径。博文包只复用
   _共享层_（`internal/rss` 不用、`internal/controller/archive`、`pkg/render`、pseudo-job 形态）。

2. **一张表 `tombkeeper_blog_post`，`category` 字段标出处，复合主键 `(category, id)`。**
   id 是站点生成的不透明 token（如 `26ho1i8FcjS`），跨 category 唯一性不由我们保证；复合主键
   `(category, id)` 天然防跨源撞键，且归档查询本来就带 category（URL 里有 slug），零额外成本。
   在 `internal/migrate/db.go` 的 AutoMigrate 注册模型。

3. **解析走 RSC flight 分片，不引 goquery。**
   仓库现无 HTML 解析库，微博即用 flight 正则 + 花括号配平+`$ref` 解引用（`tombkeeper/extract.go`）。
   博文对象更简单（`"category_slug"` 标记、无 retweet/pics/url_info），但 `content` 仍可能是 `$ref`、
   `created_at` 带 `$D` —— 需要同款 reassemble + row map + ref 解析。**在 `tkblog` 内自带一份精简
   flight 解析**（约 80 行），**不改动已测的微博 `extract.go`**。
   `// ponytail:` 注释标注：flight 通用管件（reassemble / row map / `$ref` 解引用 / `$D`）复制自
   tombkeeper/extract.go。**理由是「抽共享会迫使改动并重测微博 golden 路径」**，不是「还没第二个消费者」
   （本次上线即是第二个）；第三个 Next.js 源出现时再抽 `internal/nextflight` 共享。取 flight 而非 DOM
   的另一好处：`created_at` 是完整 ISO 时间戳（DOM 只有 `YYYY-MM-DD`）。

4. **伪 job = 复刻微博 `StartHistory` 套路，但更简。**
   `StartCrawl(category, ...)`：per-category 单飞守卫（两个 `atomic.Bool` 或按 category 键的定长
   map，仿微博 `history.go:46` 的单个 `atomic.Bool`；不用 `sync.Map`）、`xid` job_id、后台 goroutine、
   panic-recover + Bark、立即返回 `202 {job_id}`；**无 cron**。
   **每次全量**：第 1 页读 `totalPages`，`for page:=1..total` 逐页 upsert（幂等）。内容定型、总页
   数权威，故**省掉**微博 history 的「逐页复核 total、空页停」逻辑；仅保留一个兜底：`totalPages`
   解析失败时退化为「空页即停」。**无断点续传**（篇数少、幂等，崩了重触发；对齐 `tombkeeper-backfill-no-resume` 已知取舍，此处可接受）。
   `StartCrawl`/`GetPage` 入口再校验 category ∈ {xfocus, baidu}（纵深防御，防注入拼进抓取 URL）。

5. **正文存「已转义 markdown」，换行交给项目既有的 CJK 渲染约定 —— 不自造硬换行。**
   **修正（作者指出 CJK 已有处理）**：项目两套 goldmark 都**内建**东亚换行规则，不是「没开
   `WithHardWraps` 就塌成空格」：归档路径 `NewMarkdown` = `GFM + extension.CJK`（默认
   `EastAsianLineBreaksSimple`，`md2html.go:24`）—— **CJK 相邻的单 `\n` 被合并（无空格）**，非
   `<br>`、非空格；feed 路径用 `CSS3Draft`（`feed_markdown.go:20`），连 CJK/latin 边界也合并（A6
   迁移，`feed.go:31`）。微博正文正是这么存的：`escapeMarkdown` 逐行转义、以**普通 `\n`** 重拼，
   段落靠 `\n\n`，其余交给 goldmark CJK。博文 `content` 形态一致（实测样本 `\n\n` 分段）。故
   **tkblog 照抄微博**：`contentToMarkdown` = 逐行 `escapeMarkdown` + 普通 `\n` 拼接、段落 `\n\n`，
   **不加行尾两空格硬换行**（那会偏离项目 CJK 约定、把 CJK 软换行误渲染成 `<br>`）。落库为
   `text_markdown`。归档 `HandleTkblog(link)` 取库 → `{title: post.Title, markdown: body + 页脚}` →
   `htmlRender.Render`（`NewMarkdown` 路径）出 HTML，与微博归档同一条路径、同一套 CJK 行为。
   （`escapeMarkdown` 是通用纯文本转义、非 weibo 专属：copy 一份进 tkblog，或后续抽到 `internal/md` 共享。）
   页脚：`[原文链接](wayback) · [粉丝站链接](tombkeeper.io/{category}/{id})`；**`SourceURL` 为空时省掉
   原文链接**，不产出 `[原文链接]()` 空链。

6. **归档匹配粉丝站链接为主。**
   `tkblog.IsBlogArchiveLink(link)` / `BlogArchiveKey(link) (category, id)` 匹配
   `tombkeeper.io/{xfocus|baidu}/{id}`；在 `archive.go` 分派 `switch` 加一个 `case`。
   （可选，后续项：也认 wayback URL —— 按 `source_url` 反查库，本次不做。）

7. **本次不做 RSS/Atom 出口。**
   issue 只要解析/落库/伪 job/归档；内容定型，订阅价值低。故**不碰** `internal/rss`、不加 redis
   缓存键、不加 `/rss/...` 路由、不写 golden `.atom`。**若作者放行时要 RSS**，再按微波 RSS plan
   §8-9 补 `feed.go`+controller+ 路由+redis+golden 一套。

8. **日志（重点）：全程 zap、注入式。**
   `logger.With(zap.String("job_id", xid), zap.String("category", cat))` 作 job 关联；逐页
   `logger.With(zap.Int("page", n))`；Info 记生命周期/每页入库数，Warn 记跳过/解析降级，Error 记
   失败/中止；失败/panic 走 `notify.NoticeWithLogger` Bark。字段化、不用 fmt 拼接。

## 抓取层与 HTTP

`tkblog` 自带精简 fetcher（`fetch.go`）：`GET https://tombkeeper.io/{category}?page=N`，自定义
Chrome UA、重试 3 次、限速 ~1s、SSRF `CheckRedirect` 守卫 —— 仿 `tombkeeper/request.go` 的要点。
`// ponytail:` 注释：与微博 fetcher 同站、可后续统一限速；本次各自持有，避免耦合已测代码。

## 步骤（每步一提交）

### 0. 文档提交

本 issue + plan（+ 分支建好）作为 `docs(...)` 提交，先于代码。

### 1. DB 模型 + 迁移（`tkblog/db.go` + `internal/migrate/db.go`）

- 模型 `Post`：`Category`(text, PK)、`ID`(text, PK)、`Title`(text)、`CreatedAt`(timestamptz,index)、
  `TextMarkdown`(text)、`SourceURL`(text, wayback)、`CreatedDBAt`/`UpdatedDBAt`、`DeletedAt`。
  `TableName()="tombkeeper_blog_post"`。
- `DB` 接口 + `DBService struct{ *gorm.DB }` + `NewDBService`；方法 `SavePost`（**用 `d.Save(p)`**，
  对齐仓库既有复合主键先例 `github/db/repo.go` PK `(gh_user,name)` 的 `SaveRepo`；GORM v2 `Save` 在
  PK 非零时走 UPDATE、0 行则 CREATE，天然 upsert，**不自造 `OnConflict`**）、`GetPost(category, id)`、
  `PostExists(category, id)`。
- `internal/migrate/db.go`：import + AutoMigrate 注册 `&tkblog.Post{}`。
- **验证**：`go build`；本地起库建表。

### 2. flight 解析（`tkblog/extract.go`）+ 测试

- 类型 `RawArticle{ID, Category, Title, CreatedAt, URL(wayback), Content}` + `totalPages`。
- `ExtractArticles(html []byte) (arts []RawArticle, totalPages int, err error)`：reassemble flight →
  row map（`$ref` 解引用）→ 抽 `"category_slug"` 对象 → 处理 `$D`/`$ref`；顺带读 `"totalPages":N`。
- `extract_test.go`：合成含 `$D`/`$ref`/`totalPages` 的 chunked flight 串，断言篇数/字段/总页数；
  存一份真实页面片段作端到端样本（可选）。
- **验证**：`go test ./pkg/routers/tkblog/ -run Extract`。

### 3. 抓取层（`tkblog/fetch.go`）

- `Requester` 接口 + `RequestService`：`GetPage(category string, page int) ([]byte, error)`；
  UA/限速/重试/SSRF 守卫；`Close()` 停限速 goroutine。
- **URL 规则**：`page==1` 走**裸** `https://tombkeeper.io/{category}`（无 `?page=`，issue:45 明确），
  `page>=2` 才拼 `?page=N` —— 否则 `totalPages` 可能读到重定向/404 页而全盘错。
- **验证**：`go build`（构造/签名，不打网络）。

### 4. 渲染（`tkblog/render.go`）+ 测试

- `contentToMarkdown(content string) string` = 逐行 `escapeMarkdown` + 普通 `\n` 拼接、段落 `\n\n`
  （**不加硬换行**，见决策 5）；`FanSiteURL(category, id) string` = `https://tombkeeper.io/{category}/{id}`；
  `makeTitle` 直接用 `RawArticle.Title`；归档页脚拼装（`SourceURL` 空则省原文链接）。
- `buildPost(RawArticle) *Post`。
- `render_test.go`：转义/页脚断言，并**过一遍真实 HTML 渲染**（`render.NewHtmlRenderService(...).Render(...)`）
  断言与微博一致的 CJK 行为：`\n\n` → 两个 `<p>`；CJK 相邻单 `\n` **合并**（不误出 `<br>`/空格）。
- **验证**：`go test -run Render`。

### 5. 抓取编排 + 伪 job（`tkblog/crawl.go`）+ 测试

- `ingestPage(html, category, db, logger) (saved int)`：`ExtractArticles` → 逐条 `buildPost` →
  `SavePost`（幂等）。
- `crawlAll(req, db, category, logger)`：第 1 页读 total → `for page:=1..total` `ingestPage`；
  total 解析失败退化为空页停。
- `StartCrawl(db, notifier, category, logger) (jobID string, err error)`：per-category 单飞
  （占用中返回 `ErrCrawlRunning`）；`xid` job_id；后台 goroutine（panic-recover + Bark）；即时返回。
- `crawl_test.go`：stub requester/db 跑 happy-path，断言入库条数 + 幂等（重跑不增）+ 单飞拒绝。
- **验证**：`go test -run Crawl`。

### 6. 控制器 + 路由（`internal/controller/tkblog/` + `cmd/server/echo.go`）

- `controller.go`（deps: tkblog.DB, notifier, logger）+ `crawl.go` handler：路径取 `category`，
  校验 ∈ {xfocus, baidu}（trust boundary），调 `StartCrawl`；占用中 409，否则 202 带 `{job_id}`。
- 路由：admin 组 `POST /api/v1/tkblog/:category/crawl`（并入 `groupNeedAuth` → `AllowAdmin`）。
  —— 「两个伪 job」= 两个 category 各触发一次，共用一个参数化入口。
- **验证**：`go build ./...`；admin POST 两个 category，看日志抓取条数。

### 7. 归档（`tkblog/archive_link.go` + `internal/controller/archive/*`）+ 测试

- `tkblog`：`IsBlogArchiveLink(link) bool` / `BlogArchiveKey(link) (category, id string, ok bool)`
  匹配 `tombkeeper.io/{xfocus|baidu}/{id}`。
- `internal/controller/archive/tkblog_archive.go`：`HandleTkblog(link)` → `BlogArchiveKey` →
  `GetPost` → `{title: post.Title, markdown: post.TextMarkdown + 页脚}`（404 走 `ErrArchiveNotFound`）。
- `archive.go` 分派加 `case tkblog.IsBlogArchiveLink(link): return h.HandleTkblog(link)`；
  `controller.go` 注入 `tkblogDBService`。
- `archive_link_test.go`：链接匹配 parity（两 category 命中、非法/其他源不误伤）。
- **验证**：`curl /api/v1/archive/https%3A%2F%2Ftombkeeper.io%2Fbaidu%2F{id}` 返回正文 HTML。

### 8. 收口

- `just lint`；对全 diff 跑 `/code-review`（Standards + Spec），按结果修或开后续 issue；
- 更新 PROGRESS（同提交）；请作者 review → squash 合并 `master`、删分支。

## 测试（本次新增）

- `extract_test.go` —— 合成 flight（`$D`/`$ref`/`totalPages`）→ 篇数/字段/页数。
- `render_test.go` —— 转义/换行/页脚。
- `crawl_test.go` —— stub 跑 happy-path + 幂等 + 单飞拒绝。
- `archive_link_test.go` —— 归档链接匹配 parity。
- （无 golden `.atom`：本次不做 RSS。）

## 待更新文档

- [ ] `docs/ARCHITECTURE.md` —— 代码地图加 `pkg/routers/tkblog`；存储加 `tombkeeper_blog_post`。
- [ ] `docs/OPS.md` —— 加 xfocus/baidu 全量抓取触发方式（`POST /api/v1/tkblog/:category/crawl`）。
- [ ] `docs/PROGRESS.md` —— 完工时同提交追一条。
- [ ] `docs/TODO.md` —— 记「xfocus/baidu RSS 出口」与「归档认 wayback URL」为延后项。

## 后续项

- RSS/Atom 出口（若作者要）。
- 归档额外认 wayback URL（按 `source_url` 反查）。
- 若出现第三个 Next.js 源，把 flight 解析抽到 `internal/nextflight` 共享。
