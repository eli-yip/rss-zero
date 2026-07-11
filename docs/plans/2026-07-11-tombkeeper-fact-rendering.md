---
title: "tombkeeper 事实入库与读时 Markdown 渲染重写"
issue: docs/issues/2026-07-11-tombkeeper-fact-rendering.md
status: done
areas: [tombkeeper, rss, controller, migrate, db, render]
updated: "2026-07-11"
---

# PLAN: tombkeeper 事实入库与读时 Markdown 渲染重写

## 目标

解决 [issue](../issues/2026-07-11-tombkeeper-fact-rendering.md)：把 tombkeeper 的 seam 从
「抓取时产出已排版 Markdown」改成「抓取时产出事实，读取时渲染」。最终数据库足以确定性重放
现有正文，但不保存展示结果；RSS 只选择确实来自 tombkeeper.io 时间线的帖子。

## 当前症结

`Renderer` 是一个错误地放在写路径上的浅模块：调用方只想保存微博，它却同时进行网络请求、
OSS 写入、帖子写入、短链分类、引用递归、Markdown 转义和排版。其结果 `TextMarkdown` 又被 RSS
与归档当成事实来源。展示策略变化必须改写历史缓存，且 `materializePost` 的隐式写入与
`GetLatestPosts` 的无条件查询共同让非时间线引用帖有机会进入 RSS。

## 领域语言与类型名

类型名取自 [项目 glossary](../../CONTEXT.md) 中的内容概念，不取自「解析事实」「准备渲染」等处理步骤。
包名已经提供 tombkeeper 语境，不给所有类型追加 `Facts`：

| 概念                       | Go 名称                | 不采用                           |
| -------------------------- | ---------------------- | -------------------------------- |
| 来源页面中的一条微博       | `SourcePost`           | `RawPost`、`PostFacts`           |
| 已保存的博文               | `Post`                 | `StoredPost`、`PostRecord`       |
| 一页已分类的时间线内容     | `TimelinePage`         | `PageFacts`                      |
| 时间线导入模块             | `TimelineImporter`     | `Ingestor`                       |
| 导入计数                   | `ImportStats`          | `Stats`                          |
| 博文中的短链描述           | `PostLink`             | `URLInfoEntry`                   |
| 按 URL 索引的 H5 图片 id   | `Post.H5ImageIDsByURL` | `ViewPics`、`H5ImageIDs`         |
| 源图片及其获取／转存结果   | `ImageAsset`           | `Object`                         |
| 一次呈现所需的自包含内容   | `ContentSnapshot`      | `RenderDataset`、`RenderContext` |
| 内容快照装配模块           | `ContentLoader`        | `RenderDataLoader`               |
| 内容只读 adapter interface | `ContentReader`        | `FactReader`                     |

纯 renderer 不需要 `RenderOptions` 这种开放式参数袋；当前唯一环境输入直接命名为
`serverBaseURL string`。

## 关键决策

### 1. `in_timeline` 是帖子上的单调布尔事实

在 `tombkeeper_post` 增加 `in_timeline bool NOT NULL DEFAULT false`。它的含义严格限定为：
「该 id 曾在 tombkeeper.io 列表页的 `详情`条目中出现」。

- 列表页的时间线 id 写 `true`；flight 中额外携带的转发原文、详情页按需抓到的引用帖写 `false`。
- upsert 使用 `old.in_timeline OR new.in_timeline`，只允许 `false → true`，不允许降级；live crawl
  与 history 并发时也由一条原子 SQL 保证。
- RSS 查询改为 `WHERE in_timeline = true ORDER BY published_at DESC`。
- 不建 membership 表：当前只有一个固定时间线，成员身份是单调布尔值；单独建表没有额外信息。

### 2. `Post` 只保留结构化事实

重写后的 `tombkeeper_post`：

| 列                | 类型          | 含义                                                     |
| ----------------- | ------------- | -------------------------------------------------------- |
| `id`              | `bigint` PK   | 微博 mid                                                 |
| `bid`             | `text`        | 微博短 id                                                |
| `author_id`       | `text`        | 作者 uid                                                 |
| `screen_name`     | `text`        | 抓取时作者昵称                                           |
| `published_at`    | `timestamptz` | 源站发布时间                                             |
| `text`            | `text`        | 未做 Markdown 转义的原始纯文本                           |
| `pics`            | `text[]`      | 源站图片字段，保序；元素可为 pic id 或原始 URL           |
| `url_info`        | `jsonb`       | 保序的源站短链事实                                       |
| `view_pics`       | `jsonb`       | `long_url → []pic_id`；「查看图片」H5 页补全出的有序事实 |
| `retweet_post_id` | `bigint`      | 直接被转发微博 id，零值表示无；Go 字段 `RetweetPostID`   |
| `in_timeline`     | `boolean`     | 是否已在列表时间线确认                                   |
| DB 元数据         | 原类型        | `created_db_at` / `updated_db_at` / `deleted_at`         |

删除 `title`、`text_markdown`、`video_url`、`raw`：前三者可由上述事实稳定推导；`raw` 既不是应用
可直接消费的模型，又无法独立解析 flight `$ref`，不保留一个看似可回放、实际不完整的旁路。
源站时间无效时该条事实解析失败，不再用 `time.Now()` 伪造发布时间。

Go 字段 `Post.Links []PostLink`（DB 列仍为 `url_info`）原样保留源站的 `short_url` / `url_type` /
`url_title` / `long_url` 等字段。
Go 字段 `Post.H5ImageIDsByURL`（DB 列仍为 `view_pics`）记录从「查看图片」H5 页额外获取到的
`long_url → []pic_id` 索引，是结构化事实而不是展示指令。
将来源事实与获取型增强事实分列后，`url_info` 可以随新观测刷新，而一次暂时性获取失败不会用空数组
覆盖过去成功取得的 pic ids。key 使用实际请求的 H5 `long_url`，不用可能变化的 t.cn token。
两者都用 GORM JSON serializer 写 `jsonb`，不新增短链子表。

### 3. `ImageAsset` 只记录资源事实，不记录 Markdown 所有权

保留 `tombkeeper_object` 作为 pic id → OSS 获取结果：对象键、实际来源 URL、provider、成功／
放弃状态。删除无序且一图只能归属一帖的 `post_id`；帖子与图片的有序关系已经存在于
`Post.Pics` 和 `Post.H5ImageIDsByURL`。已有图片资产行继续复用，破坏性重建帖子时不重复下载全部图片。

图片获取仍发生在抓取期，但只产生 object 与 pic id 事实：不生成 `![…]`、失败提示或编号。

### 4. 时间线页面提取、时间线导入、内容快照装配、Markdown 渲染各守一个 seam

#### 页面提取

把当前外部分离的 `ExtractPosts(html)` 与 `timelineIDs(html)` 收进一个入口：

```go
ExtractTimelinePage(html []byte) (TimelinePage, error)

type TimelinePage struct {
    Entries        []SourcePost // 时间线条目，保持页面顺序
    EmbeddedPosts  []SourcePost // 内嵌博文，不与 Entries 重复
    MissingEntries int          // 有详情 id、但没有可用 SourcePost 的条目数
}
```

同一 HTML 有两份用途不同的信息：RSC flight 给出完整 `SourcePost`，但其中混有转发原文；SSR 中每个
列表条目的 `详情`链接只给出真正位于时间线的 id。模块内部先把 flight 解析成 `id → SourcePost`，
再按 `详情` id 取出 `Entries`，其余对象进入 `EmbeddedPosts`。调用方拿到的已经是分类结果，不接触
`timelineIDs`，也不负责拼接两个解析器的输出；importer 只把 `MissingEntries` 汇总进 `ImportStats`，
不会为了统计失败而重新解析 SSR。

失败策略保持“坏一帖不丢整页”，但不再伪造事实：对象 JSON 或 `created_at` 无效时只丢该对象；
若它对应时间线 id，importer 计入 failed 并继续保存同页其他帖。时间线 id 在 flight 找不到同样
逐帖 failed；只有「flight 有帖子但一个时间线详情 id 都提取不到」的整体 markup drift 才让整页返回
错误。测试锁定该策略，history 的页数推进不依赖被丢对象的发布时间。

#### 时间线导入

```go
TimelineImporter.Import(pageHTML []byte) (ImportStats, error)
```

live crawl 与 history 只调用这个 interface。其 implementation：

1. 提取时间线页面：`Entries` 以 `in_timeline=true` upsert，`EmbeddedPosts` 以 false upsert。
2. 对每个时间线根帖只补全一层直接依赖：缺页的 `RetweetPostID` 与 tombkeeper 本人「微博正文」链接
   可抓详情页并以 `in_timeline=false` 入库，不递归补全第二层引用。
3. 批量读取这些帖已存的 `H5ImageIDsByURL`。每个「查看图片」按 `long_url` 查 map：key 已存在时直接复用，
   包括值为空数组（H5 成功响应但没有图片）；key 不存在才请求 H5。请求成功便记录解析结果，哪怕
   是空数组；请求失败不写 key，让下次摄取重试。
4. 在内存里完成普通图片／新 H5 结果的补全并写 `ImageAsset`，再对每个观测到的 Post 做一次 upsert；
   不采用「先写空事实、补全后再覆盖」的两阶段顺序。
5. upsert 的冲突策略按事实种类区分：来源字段采用本次完整观测；`in_timeline` 用 SQL `OR`；
   `H5ImageIDsByURL` 以 `long_url` 为 key 合并：缺失值不删除 key，incoming 非空数组可写入 absent／空 key，
   incoming 空数组只写 absent key、不能覆盖非空数组。因而 live/history、list/detail 并发不会降低
   成员身份或资源事实。
6. 每次都 upsert 当前页事实，不再用 `PostExists` 跳过整帖；资源保存仍按 object id 幂等。

网络、OSS 与 DB 写入留在摄取 implementation 内；它没有 Markdown、标题、摘要、引用块或时间文案。
稳定状态下同一 H5 URL 只请求一次。两个进程第一次同时发现同一 URL 时允许各请求一次，最终 upsert
合并为同一事实；不为这个很短的首次竞争窗口引入跨进程锁。

#### 内容快照装配

```go
type ContentSnapshot struct {
    Posts  map[int64]Post
    Images map[string]ImageAsset
}

ContentLoader.Load(roots []Post) (ContentSnapshot, error)
```

这个读取模块注入 `ContentReader`，按两阶段装配：先把 roots 自身放进 `Posts`，由 roots 的
`RetweetPostID` / `Links` 推导并批量读取直接引用帖；再从「roots + 已读取的直接引用帖」的
`Pics` / `H5ImageIDsByURL` 收集全部 image ids 并批量读取 `ImageAsset`。这样引用块自己的图片也完整，而不会
递归加载第二层。它只装配内容，不决定 Markdown 格式。feed 的最多 50 个 roots 共用一次 snapshot，
archive 的单个 root 走同一入口，避免 N+1。

#### 纯 Markdown 渲染

```go
RenderMarkdown(postID int64, content ContentSnapshot, serverBaseURL string) (string, error)
```

这是纯函数：不持有 DB / Requester / file adapter，不读取 `config.C`、当前时间或其他全局状态。
`serverBaseURL` 的精确形状是服务 origin（如 `https://rss.example.com`，不含 `/api/v1/archive/`）；
renderer 用它构造内联归档链接。相同 `postID + content + serverBaseURL` 必须产生逐字节相同的 Markdown。
函数集中实现纯文本转义、短链展示、图片编号／失败提示、视频链接、转发和
微博正文的一层引用、原博北京时间。缺少引用或图片资产也是 snapshot 中的明确状态，按既定降级规则
渲染，不回头查询或补抓。

`BuildFeed` 读取 `LatestTimelineEntries`，一次装配 snapshot，再对每个 root 调纯 renderer，然后派生
title、summary、footer 与 HTML；归档 handler 读取 root、装配 snapshot、调同一纯 renderer，再加归档页 footer。
Redis 仍缓存 canonical items；这是明确的可丢弃缓存，不回流为数据库事实。cron 抓完后的 warm cache
也走上述读取／渲染路径。

### 5. 保留展示语义，但不保留存储字节兼容

- title 仍为纯文本折叠空白后的前 10 个 rune，只是改为读时计算。
- 普通短链、视频、「查看图片」、转发引用、微博正文引用、图片失败提示及页脚保持现状。
- 引用只展开一层；缺少被引用事实时退回普通链接／不显示引用块，不在读取时补抓。
- summary 从本次渲染出的 Markdown 提取。
- 普通图片为可抓取 id/URL 时：`ImageAsset`=OK 渲染 OSS，abandoned 或缺行渲染现有失败提示；
  非 allowlist 的完整原始 URL 仍直接嵌入。`H5ImageIDsByURL` 缺 key／空数组／没有任何 OK 图片资产时保留
  「查看图片」原始 H5 链接；有 OK 图片资产时按其中顺序编号、嵌入。读取路径不尝试补抓。
- 不承诺旧 `text_markdown` 与新 renderer 逐字节一致；承诺 fixtures / Atom golden 表达的用户可见
  语义一致。旧库中由历史字符串手术形成的偶然空行不作为兼容目标。

### 6. 采用一次破坏性重建，不做双轨兼容

新增自动 migration（下一可用版本号）。它先检查 legacy `text_markdown` 列：只有该列仍存在才在一个
DB transaction 中执行：

1. `TRUNCATE tombkeeper_post`，明确放弃旧的已渲染正文；
2. 删除 `title` / `text_markdown` / `video_url` / `raw` 与 object 的 `post_id`；
3. 保留 `tombkeeper_object` 的资源结果，供重抓复用。

新事实列先由启动期 `AutoMigrate` 建好，注册 migration 随后执行。migration 必须可重试；transaction
保证不会留下半删列状态，legacy-column guard 保证「migration 已提交、但 runner 的 applied record
写入失败」后再次启动只会 no-op 并补登记，绝不再次清空已重抓的新事实；`DROP` 同时使用
`IF EXISTS`。此发布不可回滚到依赖旧列的二进制，回滚手段只有数据库备份或重新抓取。

已经发布的 `20260709000000` / `20260710000000` migration 版本继续留在 append-only registry，
但改用 migration 内部的 legacy row struct，并在目标旧列不存在时 no-op。这样删除新 `Post` 的
`TextMarkdown` / `Raw` 字段后仍能编译；fresh DB 也能按顺序登记旧版本而不会查询不存在的列。
旧的 Markdown 字符串手术 helper 不再进入新运行时路径。

部署检查先确认新 migration 已登记完成（runner 失败不会阻止服务启动，所以这是硬门槛），然后调用
现有鉴权端点 `POST /api/v1/job/run/tombkeeper_crawl` 主动跑最新两页，确认新 cache key 已 warm 且
RSS 有内容；最后调用 `POST /api/v1/tombkeeper/history` 覆盖完整日期范围，逐页恢复历史归档。
给 tombkeeper RSS 使用新的 source cache key，避免旧 Redis canonical items 在新事实库上继续命中；
crawl warm 与 controller cache-miss 两条路径必须同时改用该 key。
不为这个过渡增加 facts version、双写或 Markdown 反向解析器。

## 代码落点

- `pkg/routers/tombkeeper/types.go`：`SourcePost` / `PostLink` / `TimelinePage`，严格时间解析。
- `pkg/routers/tombkeeper/extract.go`：单一 `ExtractTimelinePage` interface。
- `pkg/routers/tombkeeper/db.go`：内容模型、原子 upsert、`LatestTimelineEntries`、批量只读查询。
- `pkg/routers/tombkeeper/import.go`（由现 `parse.go` 重写）：`TimelineImporter` 与一层依赖／资源补全。
- `pkg/routers/tombkeeper/content.go`：`ContentLoader` 批量装配自包含 `ContentSnapshot`。
- `pkg/routers/tombkeeper/render_markdown.go`：纯 `RenderMarkdown`；删除现 `Renderer`、
  `materializePost` 及离线 Markdown 字符串手术依赖。
- `pkg/routers/tombkeeper/crawl.go` / `history.go`：只编排抓页 → import；crawl 后 warm cache。
- `pkg/routers/tombkeeper/feed.go`、`internal/controller/archive/tombkeeper_archive.go`：共用 renderer。
- `internal/migrate`：带 legacy-column guard 的事实 schema 破坏性重建 migration；保留旧版本号并让
  旧 Markdown migration 对 fresh schema 安全 no-op。

## 实施步骤（对应提交）

1. **内容模型与提取**：新增 schema 字段／DTO，合并 `ExtractTimelinePage`，实现原子 upsert 和时间线查询；
   先用测试锁定分类与不可降级。
2. **导入重写**：把图片与直接引用补全从 Markdown renderer 拆到 `TimelineImporter`，让 live/history
   只保存结构化内容。
3. **读时装配与纯渲染**：实现批量 `ContentSnapshot` loader；从 fixtures 迁移排版测试到纯
   `RenderMarkdown`，接入 feed 与 archive。
4. **破坏性迁移与运维**：添加 transaction migration、RSS cache key 隔离和完整重抓 runbook。
5. **清理**：删除 `text_markdown` 路径、旧离线字符串手术与失效测试，更新架构／SSOT／TODO，跑评审。

## 测试（必填）

- `ExtractTimelinePage`：同页时间线帖、内嵌转发原文、正文引用 id 分类正确；无效 `created_at` 只丢该对象、
  对应时间线 id 计 failed；整体详情标记漂移才报整页错误。
- DB / import：
  - extra / detail 帖以 `in_timeline=false` 保存；
  - 同 id 后续被列表页确认会 `false → true`；详情写入不能 `true → false`；
  - live 与 history 的重复摄取幂等；
  - 同一 H5 `long_url` 成功解析后重复摄取不再调用 Requester；成功空结果同样不重试，暂时性错误
    不落 key 且下次会重试；
  - 用真实 Postgres 并发两组写入，分别覆盖 `true + false` 时间线身份竞争、非空 + 空 `H5ImageIDsByURL`
    竞争，最终 `in_timeline` 与增强事实都不降级；
  - 较新的非时间线引用帖不会出现在 `LatestTimelineEntries` / RSS。
- content loader：多个 feed roots 的直接引用帖与图片资产批量装入一个 `ContentSnapshot`，特别断言
  引用帖自己的普通图片／H5 图片资产也被装入；缺失内容保持缺失，不触发网络或第二层递归。
- renderer：沿用现有 plain / image / video / retweet / original-absent / view-pic / shortlink fixtures，
  直接构造 `ContentSnapshot` 断言 Markdown 语义；编译期 interface 不接受 DB/Requester，同一输入
  重复调用输出逐字节相同，输入 snapshot 不被修改。
- feed / archive：两条路径对同一 `Post` 使用同一正文；更新 tombkeeper Atom golden，人工核对差异只来自
  已声明的不保留空行字节兼容。
- migration：至少验证 transaction 内的目标列／数据状态；在临时 Postgres 做一次启动迁移 + page 1
  重抓 smoke test；模拟 migration body 成功但 applied record 未写入，插入新事实后重跑，断言 legacy
  guard 使新事实不被二次清空；fresh DB 从空库跑完整 registry 通过。
- 纯 DB 离线重放：从 Postgres 重新装配 snapshot，再交给纯 renderer，覆盖 ImageAsset OK / abandoned /
  missing、非 allowlist 原始 URL、空 `H5ImageIDsByURL`，证明进程重启后只靠事实即可稳定渲染。
- cache：controller cache-miss 与 crawl warm 都只读写新 tombkeeper source key。
- 验证命令：`go test ./pkg/routers/tombkeeper/... ./internal/controller/archive/...`、
  相关 `internal/rss` 测试、`go build ./...`、`just lint`；不跑无关全量测试。

## 待更新文档

- [x] `CONTEXT.md`：记录博文、时间线条目、内嵌博文、图片资产与内容快照的统一语言。
- [x] `docs/ARCHITECTURE.md`：数据流改为事实入库、读时 Markdown，删除「正文已烘焙」描述。
- [x] `docs/OPS.md`：记录破坏性 migration、不可旧版本回滚、完整 history 重抓与 RSS cache key。
- [x] `docs/PROGRESS.md`：完成／发版／重抓结果与验证数字。
- [x] `docs/TODO.md`：移除依赖持久化 `title` 列的 LLM 标题表述，若保留需求则改成独立事实来源。
- [x] `pkg/routers/tombkeeper/example/README.md`：更新为事实模型与读时渲染 SSOT。
- [ ] 本 issue：合并时 `status: closed`。

## 后续项

无。实施中发现的非本方案必要改动另开 issue，不扩张本次重写。
