# LESSON: 统一 RSS 出口管线

- 日期：2026-06-24
- SPEC：[`../issues/2026-06-24-unified-rss-pipeline.md`](../issues/2026-06-24-unified-rss-pipeline.md)
- PLAN：[`../plans/2026-06-24-unified-rss-pipeline.md`](../plans/2026-06-24-unified-rss-pipeline.md)

把 7 个 RSS 源收口成一条出口管线（规范 `Item` + 唯一 `RenderAtom` + items 缓存）。记录实现中
**非显而易见**的点。

## 调研阶段就定方向的几个关键事实

- **`gorilla/feeds` 的 Atom 只输出 `<updated>`，没有 feed 级 `<published>`，entry 级
  `<published>` 也从不输出**（`newAtomEntry` 只写 `<updated>`）。`<feed><updated>` 取
  `anyTimeFormat(Updated, Created)`、**优先 Updated**。⇒ zsxq/xiaobot 渲染里那个
  `feeds.Feed.Created = time.Now()` 是**死字段**，根本不进 XML。最初担心的「请求时渲染引入
  非确定性」其实只剩 **macked 的随机 `xid` item id** 一处。**教训：动手前逐字段核对渲染库
  的实际输出，别凭字段名想当然。**
- **crawl cron 也预热同一个 redis key**（zhihu/xiaobot/github/zsxq 的 cron 抓完调 `GenerateXxx`
  渲染 XML 写缓存）。所以「缓存下沉到 items」的 blast radius 不止控制器——四个 cron 的预热步
  骤必须同改，否则与端点的新格式冲突。
- **macked / endoflife 无内容 DB**：macked 缓存即唯一持久层（cron 抓 WordPress→过滤订阅→当批
  unread），endoflife 按需爬 web。⇒ macked 冷缓存=空（部署空窗根因，靠启动预热消除）；
  endoflife 的 Fetch 就是「爬+parse」。

## 架构落点

- 规范 `Item{ID,Link,Title,Author,Time,Summary,ContentHTML}` + `FeedMeta{Title,Link,Updated}`
  - 唯一 `RenderAtom`，都在 `internal/rss`。**补了 SPEC 初稿没有的 `Summary`**：七源都设
    `feeds.Item.Description`（→ `<summary>`），不带它无法逐字节一致。
- `RenderAtom` 把 `feeds.Item.Created/Updated` 都设成 `item.Time`、feed 的 `Created/Updated`
  都设成 `meta.Updated`——这样与旧各源渲染器逐字段一致（旧的 `Created==Updated==条目时间`，
  且 feed 只 `<updated>` 进 XML）。
- **Fetch 函数的归属是混合的，不是 PLAN 初稿说的「全在 internal/rss」**：
  - zhihu/xiaobot/github/zsxq：`FetchXxx` 在 `internal/rss`（该包本就 import 这些源的 db/render）。
  - endoflife/tombkeeper/macked：`BuildFeed`/`feedFromPosts` 在**各自源包**里。原因：它们的
    数据类型（`versionInfo`、`Post`、`ParsedPost`）字段非导出，且 `internal/rss` 不 import 这
    三个包——放进 internal/rss 会够不到非导出字段、或为导出而污染 API；放源包里单向 import
    `internal/rss` 取类型即可，无环。**教训：Fetch 放哪，由「数据类型在哪 + 谁 import 谁」决定。**
- 缓存：`cachedFeed{Meta,Items}` JSON，key 加 **`v2:` 前缀**与旧 XML 物理隔离（发版即自然 miss
  重建、回滚干净）。`rss.WarmCache`（fetch+Set）给 4 个 cron 用；zhihu cron 的调用方本就有自己
  的 `redis.Set`，所以拆出 `rss.FetchCached`（只 fetch+ 序列化成字符串）复用那个 Set。

## 几处有意的行为变更（已在 SPEC 记为偏离）

- **A6 goldmark 统一**：tombkeeper 的 feed `<content>` 从 `extension.CJK`（= Simple 换行 +
  EscapedSpace 开）切到 5 源现状的 `NewCJK(CSS3Draft)`（EscapedSpace 关）。实测差异点：
  **CJK/latin 边界**的软换行 CSS3Draft 会合并（`中文\nABC`→`中文ABC`）、Simple 保留。所以
  tombkeeper `<content>` 字节会变；golden 以新配置为基准。归档页/文本渲染仍用 `NewMarkdown`，
  本期不动。
- **macked item id**：随机 `xid` → 确定性复合 `p.ID-Modified.Unix()`。随机 id 在「请求时渲染」
  下会每次轮询都变、在 Reeder 里刷屏；复合 id 既稳定（同帖未改不重复）、又在帖被 Modified 时变
  （保留旧随机 id 想要的「改动即重新冒出来」效果）。
- **zsxq Support 过滤一致化**：旧的按需 `GenerateZSXQ` **不**按 `render.Support(type)` 过滤、而
  cron 预热**过滤**——两条路径不一致。统一 Fetch 一律过滤（向 cron 看齐，读者平时看到的就是过
  滤版）。
- **random 端点**：只换渲染器（规范 Item + `RenderAtom`），其余维持现状——仍是 miss 时渲染一次、
  独立 key、24h TTL、随机 id/`time.Now()`（缓存一次故 TTL 内稳定，无刷屏）。**不进 items 缓存、
  不加 v2 前缀、不套 limit。**

## 回归策略：差分测试 → golden

- 每源迁移时先写**差分测试**（同输入分别跑旧 `Render*` 与新 `FetchXxx→RenderAtom`，断言逐字节
  相等）作为硬门槛；过了才接线、改 cron。tombkeeper 比对「除 `<content>`」、macked 比对「除
  `<id>`」（这两处有意变）。
- 清理阶段删旧渲染器时，把差分测试**转成 golden 快照**（`testdata/*.atom`，`UPDATE_GOLDEN=1`
  重生）——既能删旧代码、又保住逐字节回归门槛。`testdata/` 已加进 `.autocorrectignore`（含
  CJK+latin，autocorrect 加空格会破坏比对）。

## 工具坑

- **autocorrect 会改 Go 源码里的字符串字面量**：测试里 `[知乎-回答]墨苍离`（CJK 紧贴 ASCII 括
  号）、`中文\nABC`（CJK 紧贴 latin）会被要求加空格。对**故意无空格**的测试输入，用**字符串拼接**
  （`"中文" + "\n" + "ABC"`）让源码里不出现单个「CJK 紧贴 latin」字面量即可绕开；fixture/golden
  则加进 `.autocorrectignore`。

## 验证与遗留

- 全量 `go build ./...` 绿；7 源 golden + macked 复合 id/tombkeeper A6 属性测试 + 管线单测全过；
  改动相关控制器/render 包测试全过。
- **预存失败**（非本次引入，master 即可复现，已在 [TODO.md](../TODO.md) 记录）：
  `TestParseCycles`（endoflife `filterCycles`）、`TestExtractAnswerID`（archive）、`TestHtmlRender`
  （render snapshot）。
- 本地起服务 + 真实库/redis 的端到端（无参数逐字节对照、`limit` 切片、random、macked 冷启动预
  热）留作合并前的现网/本地验证。
