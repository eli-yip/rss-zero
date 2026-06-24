# PLAN: 统一 RSS 出口管线

- 日期：2026-06-24
- SPEC：[`docs/specs/2026-06-24-01-unified-rss-pipeline.md`](../specs/2026-06-24-01-unified-rss-pipeline.md)
- 分支：`feat-unified-rss-pipeline`

把 SPEC 拆成可独立提交、可验证的步骤。核心策略：**先建新基础设施（类型/渲染器/缓存/编排器），
再逐源迁移（每源加「差分测试」证明新渲染 == 旧渲染后才删旧代码），最后统一清理**。本 PLAN 只列
步骤、文件、接口签名、验证方式；语义细节以 SPEC 为准。

## 集成锚点（来自代码扫描）

- **路由**：`cmd/server/echo.go` `registerRSS`（353–405），`/rss` group 挂 `SetRSSContentType()`
  - `ExtractFeedID()`（后者把 `c.Get("feed_id")` 置好）。各 handler 在 `internal/controller/<source>/rss.go`。
- **task-channel 四源**：`internal/controller/common/task.go` `BuildTaskProcessor` + 各 controller
  的 `getRSS`/`generateRSS` + `internal/rss.GenerateXxx`（zhihu/xiaobot/github/zsxq）。
- **渲染**：各源 `pkg/routers/<source>/render/rss.go` 的 `RSS`/`RSSItem` + `Render*`，统一用
  `github.com/gorilla/feeds`（`Feed`/`Item`/`ToAtom()`）。共享 helper `pkg/render/rss.go`
  （`ExtractExcerpt`/`AppendOriginLink`/`BuildArchiveLink`）保留。
- **markdown→HTML**：5 源各自 `goldmark.New(GFM + NewCJK(EastAsianLineBreaksCSS3Draft))`；
  tombkeeper 用 `pkg/render.NewMarkdown()`（GFM + `extension.CJK`）。A6 统一到前者。
- **redis**：`internal/redis/redis.go` 路径常量（`ZhihuAnswerPath`/`ZsxqRSSPath`/…/`RssMackedPath`/
  `RssTombkeeperPath`）+ `RSSDefaultTTL=2h`/`RSSRandomTTL=24h`；`Set/Get/Del`。
- **crawl cron 预热点**：`zhihu/cron/crawl.go:356`、`xiaobot/cron/cron.go:139`、
  `github/cron/crawl.go:85`、`zsxq/cron/crawl.go:375`（调 `GenerateXxx`→`Set` XML）；
  `macked/crawl.go:109` `renderAndSaveRSS`；cron 注册在 `cmd/server/cron.go` `jobs`。
- **首请求副作用**：`zhihu.checkSub`、`xiaobot.checkPaper`、`github.checkRepo`（含 `/rss/github/pre`
  前缀判 `pre`）——保留在各 controller，跑在 Serve 之前。
- **random**：`internal/controller/{zhihu,zsxq}/random.go` + `pkg/routers/{zhihu,zsxq}/random/random.go`
  （构造各源 `render.RSS/RSSItem` → 调 `Render*` → `Set`，24h TTL）。
- **link 构造器保留**：`zhihu/render/link.go`（`GenerateAnswerLink` 等）、`zsxq` `BuildLink`/
  `buildGroupLink`、tombkeeper `WeiboPostURL`/`FanSiteURL` —— 迁进各 Fetch 复用，不删。

## 设计落点

- **新增包内文件，归属 `internal/rss`**（已 import 各源 DB/render，天然合适）：
  - `feed.go`：`MaxFetch=50`、`FeedMeta`、`Item`、`RenderAtom(meta, items)`。
  - `cache.go`：`cachedFeed{Meta FeedMeta; Items []Item}`（JSON）、`buildAndCache`、`sliceItems`。
  - `serve.go`：编排器 `Serve(c, ServeOptions)` + `parseLimit`/`parseType`（预留）。
  - `fetch_<source>.go`：各源 `FetchXxx(...) (FeedMeta, []Item, error)`（把旧 `Render*` 的装饰逻辑前移）。
- **共享 markdown（A6）**：`pkg/render/feed_markdown.go` —— `NewFeedMarkdown()` +
  `FeedHTML(md string) (string, error)`（包级单例，GFM + NewCJK CSS3Draft）。
- **缓存载荷换格式**：值由 Atom XML 变 `cachedFeed` 的 JSON。**为防读到旧 XML，新 key 加 `v2:`
  前缀**（如 `v2:zhihu_rss_answer_%s`）——发布即全部自然 miss 重建，无需手动清旧 key。

### 编排器签名

```go
type ServeOptions struct {
    Redis        redis.Redis
    Key          string                                   // 已 v2: 前缀
    TTL          time.Duration                            // 2h
    DefaultLimit int                                      // 缺省条数；0 = 全部 (MaxFetch)
    Fetch        func() (FeedMeta, []Item, error)         // nil = 仅读缓存、miss 渲染空 feed（macked）
    EmptyMeta    FeedMeta                                  // Fetch==nil 且 miss 时的空 feed 信封
}
func Serve(c echo.Context, o ServeOptions) error
```

`Serve` 流程：`parseLimit(c, DefaultLimit)`（缺省→default；给定→clamp `[1,MaxFetch]`）→
`buildAndCache`（命中反序列化；miss 调 `Fetch` 抓 ≤MaxFetch、序列化回填；`Fetch==nil` 则空）→
`sliceItems(items, limit)` → `RenderAtom(meta, items)` → `c.String(200, xml)`。

## 步骤（每步一提交，绿灯再下一步）

### 0. PLAN 提交（docs）

本 PLAN 作为 `docs(...)` 提交（SPEC 已先行提交 `f5e6fab`）。**PROGRESS 不在此提交**——按作者
要求随进展更新、合并前再提交终版。

### 1. 规范类型 + 共享渲染器（`internal/rss/feed.go` + `feed_test.go`）

- 定义 `MaxFetch`、`FeedMeta`、`Item`、`RenderAtom`（实现见 SPEC「规范类型与共享渲染器」，
  `feeds.Feed{Created:Updated,Updated:Updated}`、`feeds.Item{Created:Time,Updated:Time}`）。
- `feed_test.go`：构造一组 `Item` → `RenderAtom` → 断言关键 Atom 元素（`<id>/<updated>/<summary>/
  <content type="html">/<link>`）齐全且无 feed 级 `<published>`（锁信封形状）。
- **验证**：`go test ./internal/rss/ -run RenderAtom`。

### 2. 共享 feed markdown（A6）（`pkg/render/feed_markdown.go` + 测试）

- `NewFeedMarkdown() goldmark.Markdown` = `goldmark.New(WithExtensions(GFM,
  NewCJK(WithEastAsianLineBreaks(EastAsianLineBreaksCSS3Draft))))`；`FeedHTML(md) (string,error)`
  用包级单例 `Convert`。
- 测试：对含 CJK+latin+`\n` 的样本断言与「5 源现状内联 goldmark」输出一致（证明 5 源切到
  `FeedHTML` 零变化）；另记录与 `extension.CJK` 的差异样本（tombkeeper 偏离佐证）。
- **验证**：`go test ./pkg/render/ -run FeedMarkdown`。

### 3. 缓存 + 切片 + 编排器（`internal/rss/cache.go` + `serve.go` + 测试）

- `cachedFeed` JSON 序列化往返；`sliceItems(items, n)`（`n<=0`→全部；否则 `items[:min(n,len)]`）；
  `parseLimit(c, def)`（非数/≤0→def；>MaxFetch→MaxFetch）；`parseType(c)`（解析即丢，预留）。
- `Serve` + `buildAndCache`（含 `Fetch==nil` 走 `EmptyMeta` 空 feed 分支）。
- 测试：limit 缺省/越界 clamp、切片边界、`cachedFeed` 往返、空 feed 分支。
- **验证**：`go test ./internal/rss/ -run 'Serve|Slice|ParseLimit|Cache'`。

> 步骤 1–3 为纯新增、零调用方，不影响现网行为。

### 4. endoflife 迁移（最独立：无 DB、无 cron、按需爬）

- `internal/rss/fetch_endoflife.go`：`FetchEndoflife(product, redis, logger) (FeedMeta, []Item, error)`
  = `GetReleaseCycles`+`ParseCycles` → 逐 versionInfo 构造 `Item`（`ID="{product}-{ver}"`、
  `Summary=text`(markdown 源文)、`ContentHTML=FeedHTML(版本模板 text)`、标题/链接照旧）。
- `internal/controller/endoflife/rss.go`：`RSS` 改为 `rss.Serve(c, {Key:"v2:"+EndOfLifePath%product,
  TTL, DefaultLimit:0, Fetch: ()→FetchEndoflife(...)})`；删 task-channel（`getRSS`/`processTask`/
  `generateRSS`/`extractProductName`）与 `pkg/routers/endoflife` 的 `RenderRSS`/`Crawl` 写 XML 逻辑
  （`Crawl` 改为只爬+parse，或并入 Fetch）。
- **差分测试** `fetch_endoflife_test.go`：同一 `versionInfoList` 跑旧 `RenderRSS` 与
  新 `FetchEndoflife→RenderAtom`，断言逐字节相等（endoflife goldmark 本就是 CSS3Draft，无偏离）。
- **验证**：`go test ./internal/rss/ ./pkg/routers/endoflife/ -run Endoflife`；旧 `RenderRSS` 待
  步骤 12 统一删。

### 5. tombkeeper 迁移（含 A6 goldmark 切换）

- `internal/rss/fetch_tombkeeper.go`：`FetchTombkeeper(db, logger) (FeedMeta, []Item, error)`
  = `GetLatestPosts(FeedSize)` → 构造 `Item`（`ID=strconv(int64)`、`Link=WeiboPostURL`、
  `Summary=ExtractExcerpt(TextMarkdown)`、`ContentHTML=FeedHTML(TextMarkdown+"\n\n"+footer)`、
  footer 存档/粉丝站链接照旧）。**`FeedHTML` 即 A6 统一点**。
- `internal/controller/tombkeeper/rss.go`：改 `rss.Serve`（`Key:"v2:"+RssTombkeeperPath`,
  `DefaultLimit:30`, `Fetch:→FetchTombkeeper`）。cron `tombkeeper.CrawlFunc` 抓完改调
  `rss.WarmCache`（步骤含其落点，见步骤 7 注）。
- **差分测试**：tombkeeper `<content>` **以新 A6 配置为基准**（不与旧字面比）；其余字段
  （`<id>/<title>/<link>/<updated>/<summary>`）与旧 `RenderRSS` 逐字节比；另用现网样本人工
  确认 `<content>` 差异仅来自换行风格、非内容丢失（SPEC 待 PLAN #9）。
- **验证**：`go test ./internal/rss/ ./pkg/routers/tombkeeper/ -run Tombkeeper`。

### 6. macked 迁移（cron 写 items + 复合 id + 启动预热）

- `pkg/routers/macked`：`renderAndSaveRSS` 改为构造 `[]rss.Item`（`ID=fmt("%s-%d",p.ID,
  Modified.Unix())`、`ContentHTML=p.Content` 跳过 goldmark、`Summary=p.Content`、`Time=Modified`）+
  `FeedMeta{"Macked Release","https://macked.app",posts[0].Modified}` → 序列化写 `v2:RssMackedPath`。
- `internal/controller/macked/rss.go`：改 `rss.Serve`（`Key:"v2:"+RssMackedPath`, `DefaultLimit:0`,
  `Fetch:nil`, `EmptyMeta:{...}`）——miss 渲染空 feed（无 DB 回退）。
- **启动预热**：`cmd/server` 启动后 `go macked.CrawlFunc(...)()` 跑一轮（失败仅记日志）。
- **验证**：`go test ./internal/rss/ ./pkg/routers/macked/ -run Macked`；起服务确认冷启动预热后
  `/rss/macked` 有内容、复合 id 稳定（同请求两次 `<id>` 不变）。

### 7. github 迁移（含 pre + cron 预热改造）

- `internal/rss/fetch_github.go`：`FetchGitHub(subID, db, logger)` = `GetSubByIDIncludeDeleted`+
  `GetRepoByID`+`GetReleases(repo.ID,pre,1,MaxFetch)` → `Item`（`ID=%d`、标题 `buildRSSItemTitle`、
  `ContentHTML=FeedHTML("Tag: {tag}\n\n{body}")`、`Summary=ExtractExcerpt(body)`、Body/Title 回退照旧）+
  `FeedMeta`（`buildRSSFeedTitle`）。
- `internal/controller/github/rss.go`：保留 `checkRepo`（含 pre 前缀判定）→ 得 subID → `rss.Serve`
  （`Key:"v2:"+GitHubRSSPath%subID`, `DefaultLimit:20`）。删 task-channel。
- **cron 预热**（`github/cron/crawl.go:85`）：`GenerateGitHub`+`Set` 改 `rss.WarmCache(redis,
  "v2:"+GitHubRSSPath%subID, TTL, ()→FetchGitHub(subID,db,logger))`。
- **差分测试**：旧 `GenerateGitHub`(内部 `Render`) vs 新 `FetchGitHub→RenderAtom` 逐字节。
- **验证**：`go test ./internal/rss/ -run GitHub`。

> `rss.WarmCache(redis, key, ttl, fetch)` = 调 `fetch` 构 `cachedFeed` 序列化 `Set`（步骤 3 加，
> 步骤 4–10 复用）。

### 8. xiaobot 迁移

- `internal/rss/fetch_xiaobot.go`：`FetchXiaobot(paperID, db, logger)` = `GetPaper`+`GetCreatorName`+
  `FetchNPostBefore(MaxFetch, paperID, now+1h)` → `Item`（`ID=p.ID`、`Link=post URL`、
  `ContentHTML=FeedHTML(text)`（**无追链**）、`Summary=ExtractExcerpt(text)`）。
- controller 保留 `checkPaper` → `rss.Serve`（`DefaultLimit:20`）；删 task-channel；
  cron（`xiaobot/cron/cron.go:139`）改 `rss.WarmCache`。
- **差分测试** + `go test ./internal/rss/ -run Xiaobot`。

### 9. zhihu 迁移（answer/article/pin + 删 calculateTime）

- `internal/rss/fetch_zhihu.go`：`FetchZhihu(contentType, authorID, db, logger)` = 按类型
  `GetLatestN*(MaxFetch,authorID)` → `Item`（`ID=%d`、`Link=BuildArchiveLink(serverURL,官网链接)`、
  `ContentHTML=FeedHTML(AppendOriginLink(text,官网链接))`、`Summary=ExtractExcerpt(text)`、pin 标题
  回退 `strconv(id)`）+ `FeedMeta`（`[知乎-{类型}]{author}` + profile；空=1970）。**删 `calculateTime`**
  （现恒等）→ `Time` 直用 `CreateAt`。
- controller 三个 handler（Answer/Article/Pin）各保留 `checkSub` → `rss.Serve`（`DefaultLimit:20`）；
  删 task-channel；cron（`zhihu/cron/crawl.go:356`）改 `rss.WarmCache`。
- **差分测试**：旧 `Render`（calculateTime 现恒等）vs 新逐字节（三类型各一）。
- **验证**：`go test ./internal/rss/ -run Zhihu`。

### 10. zsxq 迁移（含 Support 过滤一致化）

- `internal/rss/fetch_zsxq.go`：`FetchZSXQ(groupID, db, logger)` = `GetLatestNTopics(groupID,MaxFetch)`
  → **按 `render.Support(topic.Type)` 过滤**（向 cron 预热看齐，SPEC 待 PLAN #4）→ `Item`
  （`ID=FakeID||strconv(topicID)`、`Link=BuildArchiveLink(serverURL,官网链接)`、
  `ContentHTML=FeedHTML(AppendOriginLink(text,官网链接))`、标题回退 `strconv(topicID)`）。
- controller → `rss.Serve`（`DefaultLimit:20`）；删 task-channel；cron（`zsxq/cron/crawl.go:375`）
  改 `rss.WarmCache`（其 `buildRSSTopic` 已过滤，行为对齐）。
- **差分测试**：注意旧按需 `GenerateZSXQ` **不过滤**、cron 预热 **过滤**；新统一为过滤——差分以
  **cron 预热版（过滤后）** 为基准；按需路径的过滤是有意修正（记入 LESSON）。
- **验证**：`go test ./internal/rss/ -run ZSXQ`。

### 11. random 端点迁移 + 删各源 RSS 结构体

- `pkg/routers/zhihu/random/random.go`、`zsxq/random/random.go`：改为构造 `[]rss.Item`
  （沿用 `rand`/`xid` 作 ID、`time.Now()` 作 Time）+ `FeedMeta` → `rss.RenderAtom`；缓存/24h TTL/
  单选逻辑不变（仍 miss 渲一次 `Set`）。
- 此时各源 `render` 的 `RSS`/`RSSItem`/`Render*` 已无调用方，**删除 RSS-envelope 符号**
  （仅 `rss.go` 内：zhihu/xiaobot/github/zsxq/tombkeeper/endoflife/macked 的 `RSS`/`RSSItem`/
  `RSSRender*`/`Render`/`RenderEmpty*`/`calculateTime`/`defaultTime`/title&link 私有 helper 中已迁走者）；
  **保留** link.go / full_text.go / markdown.go / models.go / html*.go 等被 export/refmt/parse/cron
  复用的部分（github/render 仅 rss.go，整包删）。
- **验证**：`go build ./...`、`go test ./pkg/routers/{zhihu,zsxq}/random/`。

### 12. 清理与下线

- 删 `internal/rss.GenerateZhihu/GenerateZSXQ/GenerateXiaobot/GenerateGitHub`（及 `internal/rss/
  {zhihu,xiaobot,github,zsxq}.go` 中已被 fetch_*.go 取代者）。
- 删各 controller 的 task-channel 残留（`getRSS`/`processTask`/`generateRSS`/`RssGenerator`），
  macked controller 退化的 `taskCh`/`processTask`；`internal/controller/common/task.go`
  `BuildTaskProcessor` 若全无引用则删。
- `grep` 确认无悬挂引用；`just lint`（dprint + golangci）零 issue；`go build ./...`。
- **验证**：`go vet ./...` + 全量 `go build`；定向 `go test ./internal/rss/... ./internal/controller/...`。

### 13. 端到端回归 + LESSON + PROGRESS 终版

- 本地起服务 + 真实库/redis：对 7 源各请求**无参数**与 `?limit=N`，确认无参数与改造前一致
  （tombkeeper 仅 `<content>` 允许差异、人工核），`limit` 切片正确、同一份缓存零碎片
  （不同 limit 不增 key）；random 两端点正常；macked 冷启动预热有内容。
- 整理 `docs/lessons/2026-06-24-01-unified-rss-pipeline.md`（差分测试经验、A6 偏离实测、zsxq
  过滤修正、v2 key 迁移、复合 id）。
- **PROGRESS 终版**：合并前一次性更新 `docs/PROGRESS.md` 状态。

## 风险与回滚

- **v2 key 前缀**让新旧缓存物理隔离：异常时回滚代码即恢复旧 XML 缓存路径，互不污染。
- 每源差分测试为硬门槛，未过不删旧渲染器——任一源出问题可单独留在旧路径（旧 `GenerateXxx` 在
  步骤 12 前一直可用）。
- A6 仅影响 tombkeeper `<content>`；若实测换行差异不可接受，可单独让 tombkeeper Fetch 用
  `render.NewMarkdown`（保留旧字节），不影响其余 6 源。

## 非目标（同 SPEC）

不改路由/中间件；goldmark 统一仅限 feed ContentHTML；不动 `/api/v1/feed/*`、crawl/parse 入库、
random 抽取/缓存/TTL 语义；`type` 仅解析占位。
