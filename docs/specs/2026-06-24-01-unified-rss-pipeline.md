# SPEC: 统一 RSS 出口管线

- 日期：2026-06-24
- 状态：已定稿（作者已评审通过），进入 PLAN（含 A6 goldmark 统一）

## 背景

现在 7 个 RSS 源各有各的生成与渲染逻辑：

- **生成入口三种执行模型**：
  1. **task-channel 四源**（zhihu / xiaobot / github / zsxq）：控制器把 redis key 投进
     task 通道，processor 命中即返回、未命中调 `internal/rss.GenerateXxx` 从 DB 生成
     XML 并回填 redis。
  2. **tombkeeper**：控制器直接读 redis，miss 时从 DB 重渲染并回填（自愈）。
  3. **endoflife**：无 DB，task-channel miss 时**按需爬 web** → 渲染 XML 直接写 redis。
  4. **macked**：无内容 DB，仅整点 cron 渲染 XML 写 redis；控制器 miss 时**返回空串**。
- **6+ 套渲染结构体**：`zhihu/render.RSS`、`xiaobot/render.RSS`、`github/render.RSSItem`、
  `zsxq/render.RSSItem`、tombkeeper 直接用 `Post`、endoflife `versionInfo`、macked
  `ParsedPost`，每套各有一份 `Render*`，都用 `gorilla/feeds` 拼 `feeds.Feed/Item` →
  `ToAtom()`，信封逻辑高度重复但各写各的。
- **缓存的是渲染后的 XML**：所有源都把渲染好的 Atom XML 存 redis（`RSSDefaultTTL=2h`；
  random 端点 `RSSRandomTTL=24h`），limit 概念不存在——条数硬编码在生成时。

**目标**：在 `/rss/<source>` 这一层收口成一条**统一出口管线**——拆分 fetch / render，
定义唯一的规范条目类型与共享 Atom 渲染器，缓存从「渲染后 XML」下沉到「条目（items）」，
并支持 `limit` 查询参数（预留 `type` 等）。**无参数时行为与改造前逐字节一致**是回归验收基线。

### 现状核实（决定设计的关键，均已读码确认）

1. **`/rss` 路由与中间件**（`cmd/server/echo.go` `registerRSS`，约 353–405 行）：
   `/rss` group 挂 `SetRSSContentType()` + `ExtractFeedID()` 两个中间件。各路由：
   - zhihu 走**路径** `/rss/zhihu/{answer,article,pin}/:feed` + `/rss/zhihu/random`；
   - zsxq `/rss/zsxq/:feed` + `/rss/zsxq/random`；
   - xiaobot `/rss/xiaobot/:feed`；endoflife `/rss/endoflife/:feed`；
   - macked `/rss/macked` **与** `/rss/macked/:feed`（bare + :feed 两条，:feed 仅为套
     `ExtractFeedID` 中间件，内容相同）；tombkeeper 同形（bare + :feed）；
   - github `/rss/github/:feed` + `/rss/github/pre/:feed`（pre 由 URL 前缀判定）。

2. **生成器 / 渲染包不止 `/rss` 用——这是缓存下沉的真正 blast radius**：
   `internal/rss.GenerateXxx` 与各源 `render` 包除被 `/rss` 控制器调用外，**还被 crawl
   cron 调用**，在抓取完成后把渲染好的 XML 预热写进**同一个 redis key**：
   - `pkg/routers/zhihu/cron/crawl.go:356`、`xiaobot/cron/cron.go:139`、
     `github/cron/crawl.go:85`、`zsxq/cron/crawl.go:375`。
   - 即 `/rss` 平时命中的是 **cron 预热的缓存**，按需 task-channel 生成只是 miss 回退。
   - 推论：缓存改存 items 后，这四个 cron 的预热步骤会与端点的新格式冲突，**必须一并改**。

3. **渲染下沉到「请求时」引入的非确定性——已逐字段核实 gorilla/feeds v1.2.0**：
   - **Atom 输出无 feed 级 `<published>`，entry 级 `<published>` 也从不输出**
     （`atom.go` `newAtomEntry` 只写 `<updated>`；`AtomFeed` 只有 `<updated>`）。
     feed `<updated> = anyTimeFormat(Updated, Created)`，**优先 `Updated`**。
   - 故 zsxq / xiaobot 渲染里 `feeds.Feed.Created = time.Now()` 是**死字段、不进 XML**
     （它们 `Updated = items[0].CreateTime`，确定性）。**虚惊一场**。
   - 每条目 `Created` 也仅作 `<updated>` 的零值回退，各源都已显式设 `Updated`，故 inert。
   - **真正的非确定性只有一处：macked 的 `Id = xid.New()`（随机）**。
   - zhihu 的 `calculateTime` hack 现已恒等于原时间（当前日期已过 2024-06-22 阈值），
     可安全删除，无行为变化。

4. **macked 随机 id 的来由**（git 考古 `d0811a6/e31a18b/aa98c9b/15f632d`）：
   macked feed 是**增量推送**（cron 每小时只渲染本批 unread 帖、无新帖渲染空），随机 id
   是为让**被 Modified 的原帖**在 Reeder 5 里重新冒出来（Reeder 按 `<id>` 去重、不因
   `<updated>` 变化重标未读）。该 hack 依赖「每批只渲染一次」（旧模型每小时渲一次、缓存
   2h）。**搬到「请求时渲染」后随机 id 会反噬**：同一批缓存帖每次轮询都得到新 id，在
   Reeder 里反复当新条目刷屏。

5. **规范条目缺一个 summary 维度**：七源的 `feeds.Item` 都设了 `Description`（→ Atom
   `<summary>`），规则不一：zhihu/zsxq/xiaobot/github/tombkeeper = `ExtractExcerpt`
   原文前 100 字、endoflife = markdown 源文全文、macked = 全文 HTML。要逐字节一致，
   规范 Item 必须带 `Summary`，由各源 Fetch 算好。

6. **markdown→HTML 有两套 goldmark 配置**：tombkeeper 用 `render.NewMarkdown()`
   （GFM + `extension.CJK` = `NewCJK(WithEastAsianLineBreaks(), WithEscapedSpace())` →
   **East Asian 换行 Simple + EscapedSpace 开**），其余五源用
   `goldmark.New(GFM + NewCJK(EastAsianLineBreaksCSS3Draft))`（→ **换行 CSS3Draft +
   EscapedSpace 关**）。**作者指示在本期统一成一套**（见决策 A6）：全部 feed ContentHTML
   收口到一个共享构造器、采用 5 源现状那套（CSS3Draft / EscapedSpace 关），tombkeeper 由
   `extension.CJK` 切换过来。**这会改变 tombkeeper `<content>` 字节输出**（微博正文 CJK+latin+
   `\n` 混排会命中换行风格差异），属对逐字节基线的有意偏离（见「回归验收基线」）。

7. **首请求副作用必须保留**（在 Fetch 之前）：zhihu `checkSub`、xiaobot `checkPaper`、
   github `checkRepo`（含 pre 处理）会在首次请求时访问对应站点校验并**自动建订阅**；
   不存在/校验失败返回 `400`。zsxq / endoflife / macked / tombkeeper 无此前置。

8. **endoflife / macked 无内容 DB 回退**：endoflife 的「Fetch」=爬 web+parse；macked 的
   缓存就是唯一持久层（cron 抓 WordPress → 过滤订阅 → 当批 unread）。故 macked 冷缓存=空
   （部署空窗根因）；tombkeeper 因有 DB 不受影响。

9. **硬编码条数**：`config.DefaultFetchCount=20`（zhihu/xiaobot/zsxq）、github 硬编码
   `GetReleases(...,1,20)`、tombkeeper `FeedSize=30`、macked WordPress `per_page=30`、
   endoflife 渲染全部 versionInfo。

10. **zsxq 既存小不一致**：cron 预热路径 `buildRSSTopic` 会按 `render.Support(topic.Type)`
    过滤不支持的话题类型，而按需 `internal/rss.GenerateZSXQ` **不过滤**。统一 Fetch 需选定
    一种（见「待 PLAN」）。

## 已确认的设计决策（既定方向 + 作者答复）

> 以下为作者已拍板项，SPEC 据此定稿。

**既定方向（作者纳入，不再征求）：**

- **A1** 保留现有 `/rss/*` 全部路径与语义，只在 `/rss` 层加统一参数解析层；无参数时与改造前
  逐字节一致。zhihu 的 answer/article/pin 仍走路径，不挪 query。
- **A2** 拆 fetch / render：定义规范条目类型 `Item`（ID 用 `string` 吸收各源 int/int64/xid
  差异）与唯一共享 Atom 渲染器，取代 6+ 套结构体与各 `Render*`。
- **A3** ContentHTML 在各源 **Fetch 阶段**算好：markdown→HTML 及各源 body 装饰（zhihu/zsxq
  追加原文链接、zsxq/tombkeeper 归档代理链接、tombkeeper 粉丝站链接、github「Tag:」前缀、
  endoflife 版本模板文本）全部留在各自 Fetch；出口渲染器只套信封、不做任何内容加工。
  macked 内容本就是 WordPress HTML，跳过 goldmark。
- **A4** 缓存下沉到 items（不再缓存渲染后 XML）：Fetch 一律抓取至多 **MaxFetch** 条整体缓存，
  出口按 limit 切片；**limit 不进 cache key**，任意 limit 命中同一份缓存、零碎片。limit 缺省
  时各源沿用原默认条数。
- **A5** 全部 7 源一次到位，把三种执行模型拍平成同一条线性管线；macked cron 从「渲染 XML 写
  redis」改为「缓存 items 写 redis」。
- **A6**（作者于 SPEC 评审追加）**goldmark 配置统一成一套**：把现有两套 feed-content
  markdown 配置收口到**一个共享构造器**，采用其余 5 源现状那套
  （`GFM + NewCJK(EastAsianLineBreaksCSS3Draft)`，EscapedSpace 关），**tombkeeper 切换过来**
  （放弃 `extension.CJK` 的 Simple 换行 + EscapedSpace 开）。macked 仍跳过 goldmark。
  代价：tombkeeper `<content>` 字节输出会变，是对逐字节基线的有意偏离（见下）。
  > 范围仅限 **feed ContentHTML**；归档页/文本渲染仍用各自的 `render.NewMarkdown`，本期不动。

**作者答复（本次讨论）：**

- **Q1 → MaxFetch = 50**。（≥ 现有最大默认 30，留约 1.6× 余量；单 feed 缓存体积小。）
- **Q2 → 随机端点仅复用共享渲染器**：`/rss/zhihu/random`、`/rss/zsxq/random` 迁到规范 Item +
  共享渲染器（借此删掉各源 RSSItem/RSS 结构体），其**抽取逻辑 / 缓存 / 24h TTL 维持现状**，
  **不套 limit/切片**。
- **Q3 → macked 启动时预热跑一轮**：服务启动后异步触发一次 macked 抓取，消除部署空窗。
- **Q4 → macked id 改确定性复合 `p.ID + Modified`**：同帖未改→id 不变（消除请求时渲染会引入
  的每轮询刷屏 bug）；同帖被 Modified→id 变（保留旧随机 id 想要的「改动即在 Reeder 重新出现」
  效果）；可做 golden 回归。
- **Q5 → DB 源 cron 改为重新预热 items 缓存**：zhihu/xiaobot/github/zsxq/tombkeeper 的 crawl
  cron 抓完后调统一 Fetch 写 items 缓存（1:1 替换现在「渲染 XML 写缓存」那步），缓存始终热、
  无首请求延迟，行为最贴近现状。

## 目标

1. 定义规范条目类型与唯一共享 Atom 渲染器，删除 6+ 套渲染结构体与各 `Render*`。
2. 在 `/rss` 层新增统一参数解析（`limit`，预留 `type`），无参数时与改造前**逐字节一致**。
3. 缓存从「渲染后 XML」下沉到「条目 items」：Fetch 至多 MaxFetch=50 条整体缓存（limit 不进
   key），出口按 limit 切片再渲染。
4. 7 源 fetch/render 拍平到同一条线性管线；含 cron 预热改为缓存 items、macked 确定性 id +
   启动预热、random 端点复用共享渲染器。

## 规范类型与共享渲染器

```go
// FeedMeta 为 feed 级信封；Updated 即 Atom <feed><updated>，唯一会进 XML 的 feed 时间。
type FeedMeta struct {
    Title   string
    Link    string
    Updated time.Time
}

// Item 为规范条目；各源 Fetch 阶段产出，渲染器只照搬、不加工。
type Item struct {
    ID          string    // Atom <id>；string 吸收 int/int64/xid 差异
    Link        string    // Atom <link href>
    Title       string    // Atom <title>
    Author      string    // Atom <author><name>
    Time        time.Time // Atom <updated>（= 旧 Created = Updated）
    Summary     string    // Atom <summary>（旧 feeds.Item.Description）
    ContentHTML string    // Atom <content type=html>（旧 feeds.Item.Content）
}

// RenderAtom 是唯一出口渲染器。
func RenderAtom(meta FeedMeta, items []Item) (string, error)
```

`RenderAtom` 内部：

```go
feed := &feeds.Feed{
    Title: meta.Title, Link: &feeds.Link{Href: meta.Link},
    Created: meta.Updated, Updated: meta.Updated, // Created inert，设成 Updated 保字节一致
}
for _, it := range items {
    feed.Items = append(feed.Items, &feeds.Item{
        Title: it.Title, Link: &feeds.Link{Href: it.Link},
        Author: &feeds.Author{Name: it.Author}, Id: it.ID,
        Description: it.Summary, Created: it.Time, Updated: it.Time,
        Content: it.ContentHTML,
    })
}
return feed.ToAtom()
```

**逐字节一致论证**：各源现有 `feeds.Item` 都设 `{Title, Link, Author, Id, Description,
Created, Updated, Content}` 这八个字段，且 `Created == Updated == 条目时间`（zhihu 的
`Updated=calculateTime` 现已恒等、可删）。feed 级仅 `<updated>` 进 XML，由 `meta.Updated`
决定。因此把每源 Fetch 产出的 `(FeedMeta, []Item)` 喂给 `RenderAtom`，输出与旧各源渲染器
**逐字段一致**——唯一例外 macked 的随机 id 改确定性复合 id（旧值本就每渲染必变、不可比）。

## 统一出口管线（请求路径）

```
解析参数(limit, 预留 type)
  → [源前置] 确保订阅 / 解析 feed id      // zhihu checkSub / xiaobot checkPaper / github checkRepo(pre)；其余无
  → 取或建 items 缓存                      // 命中即用；miss 调该源 Fetch() 抓 ≤MaxFetch 条 + 回填缓存
  → 按 limit 切片 items[:n]
  → RenderAtom(meta, items[:n])
  → c.String(200, xml)
```

- **缓存载荷**：每源缓存 `{meta: FeedMeta, items: []Item}`（JSON 序列化），key 沿用现有
  redis 路径常量（`zhihu_rss_*` / `xiaobot_rss_*` / `github_rss_*` / `zsxq_rss_*` /
  `endoflife_rss_*` / `macked_rss` / `tombkeeper_rss`），**limit 不进 key**。TTL 仍 2h。
  > 缓存载荷由 XML 变 JSON，旧 XML 缓存与新格式不兼容——发布时新 key 自然 miss 重建，无需
  > 手动清；细节（是否换 key 前缀以防误读旧值）留 PLAN。
- **切片与默认**：`limit` 缺省→源默认；`limit` 给定→clamp 到 `[1, MaxFetch]`。`meta` 从缓存
  载荷直接取（不随切片变；各源 meta 取自最新条目/静态信息，切 `items[:n]` 不影响 `items[0]`）。
- **源前置副作用**保持原位、原语义（含失败 `400`），只是后续从「读 XML 缓存」变「读 items
  缓存 / Fetch」。

### MaxFetch 与默认条数

- `MaxFetch = 50`，新增常量（替代散落的硬编码）。
- 缺省 limit：zhihu/xiaobot/zsxq/github = **20**；tombkeeper = **30**；macked / endoflife =
  **全部缓存条目**（≤ MaxFetch）。
  > macked 当批 unread ≤ WP `per_page=30`、tombkeeper `FeedSize=30`、endoflife 现实产品
  > 版本数一般 < 50，故 MaxFetch=50 不截断现有任何源（endoflife 极端产品超 50 的兜底留 PLAN 核实）。

## 各源 Fetch 适配（逐源）

下表为各源 `Fetch()` 应产出的 `(FeedMeta, []Item)` 映射（即把现有渲染器逻辑前移到 Fetch、
内容装饰原样保留）。「goldmark」列标明该源 ContentHTML 用哪套配置（保持现状、不统一）。

| 源                       | 数据来源                             | FeedMeta(Title / Link / Updated)                                             | Item.ID                   | Item.Link                               | Item.Summary                   | ContentHTML 装饰                                            | goldmark           | 默认 |
| ------------------------ | ------------------------------------ | ---------------------------------------------------------------------------- | ------------------------- | --------------------------------------- | ------------------------------ | ----------------------------------------------------------- | ------------------ | ---- |
| zhihu answer/article/pin | DB `GetLatestN*(20)`                 | `[知乎-{类型}]{author}` / profile / `rs[0].CreateTime`（空=1970）            | `%d`(id)                  | `BuildArchiveLink(serverURL, 原文链接)` | `ExtractExcerpt(text)`         | `goldmark(AppendOriginLink(text, 原文链接))`                | NewCJK             | 20   |
| xiaobot                  | DB `FetchNPostBefore(20, …, now+1h)` | `{paperName}` / `xiaobot.net/p/{id}` / `rs[0].CreateTime`（空=1970）         | `p.ID`                    | `xiaobot.net/post/{id}`                 | `ExtractExcerpt(text)`         | `goldmark(text)`（无追链）                                  | NewCJK             | 20   |
| zsxq                     | DB `GetLatestNTopics(20)`            | `{groupName}` / `wx.zsxq.com/group/{id}` / `items[0].CreateTime`             | `FakeID` 或 `%d`(topicID) | `BuildArchiveLink(serverURL, 官网链接)` | `ExtractExcerpt(text)`         | `goldmark(AppendOriginLink(text, 官网链接))`                | NewCJK             | 20   |
| github(+pre)             | DB `GetReleases(…,1,20)`             | `[GitHub]{Repo}[-WithPre]` / `rs[0].Link` / `rs[0].UpdateTime`（空=1970）    | `%d`(id)                  | `release URL`                           | `ExtractExcerpt(Body)`         | `goldmark("Tag: {tag}\n\n{body}")`                          | NewCJK             | 20   |
| tombkeeper               | DB `GetLatestPosts(30)`              | `tombkeeper 微博` / `weibo.com/u/1401527553` / `posts[0].PostTime`           | `%d`(int64 id)            | `WeiboPostURL(uid,bid,id)`              | `ExtractExcerpt(TextMarkdown)` | `goldmark(TextMarkdown + "\n\n" + [存档链接]·[粉丝站链接])` | **统一 (NewCJK)**⚠️ | 30   |
| endoflife                | 爬 web+parse                         | `Title({product} Release)` / `endoflife.date/{product}` / `v[0].releaseDate` | `{product}-{ver}`         | `endoflife.date/{product}`              | **markdown 源文** text         | `goldmark(版本模板 text)`                                   | NewCJK             | 全部 |
| macked                   | cron 抓 WP+ 过滤                     | `Macked Release` / `macked.app` / `posts[0].Modified`                        | **`{p.ID}-{Modified}`**   | `p.Link`                                | `p.Content`（HTML 全文）       | `p.Content`（**跳过 goldmark**）                            | —                  | 全部 |

补充语义（保持现状）：

- zhihu pin / zsxq topic 的 `Title` 空时回退 `strconv(id)`；github `Title` 空回退 `Tag`、
  `Body` 空回退 `RawBody`；item 标题套 `buildRSSItemTitle`（`{Repo}: {title}`，pre 加
  `[Pre-Release]`）。Author：zhihu/zsxq/xiaobot=作者名、github=RepoName、tombkeeper=
  ScreenName、endoflife=`EndOfLife`、macked=`Macked`。
- **空 feed**：各源沿用现有 `RenderEmpty*` 的 meta（zhihu/xiaobot/github=1970；tombkeeper/
  macked 旧用 `time.Now()`——空 feed 极少见，其 `<updated>` 的 `time.Now()` 作为可接受边界
  非确定性保留，或在 PLAN 统一改 1970，二选一留 PLAN）。

### random 端点（仅复用共享渲染器）

- `/rss/zhihu/random`、`/rss/zsxq/random` 保留独立 handler、独立 redis key、24h TTL、
  单选 1 条、`time.Now()`/随机 id 输入——**全部维持现状**。
- 仅把内部「构造各源 `render.RSS`/`RSSItem` → 调各源 `Render`」改为「构造规范 `Item` →
  `RenderAtom`」，从而可删掉各源 `RSSItem/RSS` 结构体与 `Render*`。
- 因仍是 miss 时渲染一次、缓存 24h，随机输入在 TTL 内稳定，无请求时刷屏问题。**不套 limit**。

### cron 与 macked 预热

- **DB 源（zhihu/xiaobot/github/zsxq/tombkeeper）crawl cron**：把现在「调 `GenerateXxx` 渲染
  XML → `redis.Set`」改为「调统一 `Fetch()` → 缓存 items」，1:1 替换，缓存保持热。
- **macked cron**：把 `renderAndSaveRSS`（渲染 XML 写 redis）改为「缓存 items（unread 批，
  确定性复合 id）写 redis」；语义仍是增量推送（缓存即当批 unread）。
- **macked 启动预热**：服务启动后**异步**触发一次 macked 抓取（消除部署到首个整点 cron 间的
  空窗）；失败仅记日志、由下个整点 cron 兜底（不阻塞启动）。
- **endoflife**：无 cron，维持按需——miss 时 Fetch=爬 web+parse、缓存 items。

## 回归验收基线（逐字节 / golden）

- **无参数请求**对每源产出的 Atom XML 与改造前**逐字节一致**为硬验收标准，**两处有意例外**：
  1. **macked**：id 由随机 xid 改确定性复合 id（旧值本就每渲染必变、不可比），按新规则断言。
  2. **tombkeeper `<content>`**：因 goldmark 统一（A6）从 `extension.CJK` 切到
     `NewCJK(CSS3Draft)`，East Asian 换行风格与 EscapedSpace 处理变化，HTML 会变；
     tombkeeper golden 以**统一后新配置**的输出为准，不与旧字面比。
- 其余 5 源 + macked 的非 content 字段：因移除随机/`time.Now()` 影响后输出确定（空 feed 的
  `time.Now()` 边界除外），可对典型数据快照做 **golden 文件**回归。
- 验证方式（细节留 PLAN）：用现网/本地真实数据，改造前后各请求一次同一 feed，diff 为空（
  tombkeeper 仅允许 `<content>` 差异且需人工确认差异来自换行风格、非内容丢失）。

## 非目标

- 不改 `/rss/*` 任何路由、路径语义或中间件（`SetRSSContentType` / `ExtractFeedID`）。
- goldmark 统一仅限 **feed ContentHTML**（A6）；归档页 / 文本渲染用的 `render.NewMarkdown`
  本期不动。
- 不改各源 crawl/parse 入库逻辑，不动 `/api/v1/feed/*`（走 FreshRSS、与本管线无关）。
- 不改 random 端点的抽取/缓存/TTL 语义。
- 不引入持久化的 limit/分页游标；limit 仅在已缓存的 ≤MaxFetch 条上切片。
- 本期 `type` 参数只做**解析占位**（校验/忽略），不接任何实际分流。

## 待 PLAN 阶段细化的点

1. **参数解析层形态**：中间件（仿 `ExtractFeedID`）把 `limit`（及预留 `type`）解析入 context，
   还是共享 helper 在切片处取——以及非法 `limit` 的 clamp/报错策略。
2. **缓存载荷与 key**：`{meta, items}` 的 JSON schema；是否给新格式换 key 前缀以防读到旧 XML；
   序列化体积与 TTL 复核。
3. **管线抽象落点**：定义统一 `Fetch(ctx) (FeedMeta, []Item, error)` 接口与各源适配器的包归属
   （`internal/rss` 收口还是新建 `internal/rss/pipeline`）；task-channel 是否保留/简化（macked
   控制器的 task 通道已近乎退化）。
4. **zsxq 类型过滤一致性**：统一 Fetch 是否套 `render.Support(topic.Type)`（向 cron 预热看齐）。
5. **endoflife MaxFetch 兜底**：核实是否有现实产品版本数 > 50，决定是否豁免该源的截断。
6. **空 feed 的 `time.Now()`**：tombkeeper/macked 空 feed `<updated>` 是否统一改 1970 以彻底可
   golden。
7. **golden 回归夹具**：选取各源代表性数据快照、改造前后 diff 的具体执行方式与放置位置。
8. **删除清单**：6+ 套 `render.RSS/RSSItem` 与 `Render*`、`internal/rss.GenerateXxx`、各源
   重复信封代码的下线次序（确保 cron / random / export 等所有调用方同步迁移，无悬挂引用）。
9. **goldmark 统一落地（A6）**：抽共享 feed-content 构造器（`GFM + NewCJK(CSS3Draft)`）的归属
   与命名；tombkeeper 切换点；核实 EscapedSpace 由开→关、换行 Simple→CSS3Draft 对 tombkeeper
   既有正文的实际差异（用现网样本跑 diff 人工确认非内容丢失）；记录 tombkeeper feed 与其归档页
   （仍走 `NewMarkdown`）渲染风格将出现的轻微分叉。
