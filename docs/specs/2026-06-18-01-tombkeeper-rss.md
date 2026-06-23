# SPEC: tombkeeper 微博 RSS 订阅源

- 日期：2026-06-18
- 状态：已定稿（作者已评审通过），进入 PLAN 与实施

## 背景

`tombkeeper`（微博 UID `1401527553`）是安全圈知名博主。其微博无官方稳定 RSS，
而第三方镜像站 `https://tombkeeper.io` 已把他的微博按时间线整理成网页。我们要
基于该镜像站，在 rss-zero 里新增一个 **tombkeeper 微博订阅源**，对齐项目里已有
的单源订阅（如 `macked`）的形态：硬编码每小时抓取、单 feed、无需 cookie、对外
暴露 `/rss/tombkeeper`。

### 镜像站事实调研（决定设计的关键）

`tombkeeper.io` 是一个 Next.js SSR 站点：

- 列表页 `https://tombkeeper.io/?page=N`，**每页 5 条**，翻页用 `?page=2`，约 6558 页。
- 每条微博的**结构化数据**嵌在页面 `<script>` 的 RSC flight 负载里，字段如下
  （实测 38 条样本）：

  ```json
  {
    "id": "5310428819230540",            // 微博数字 id（detail 链接用）
    "bid": "R4niFCJhG",                   // mblogid（短 id）
    "user_id": "1401527553",
    "screen_name": "tombkeeper",
    "text": "佛得角全国人口约 50 万…\n#佛得角官方感谢中国帮圆梦世界杯#",  // 纯文本，非 HTML
    "article_url": "",
    "topics": "", "at_users": "",
    "pics": "53899d01ly1ie5wrym85ej20sg0gl41f,53899d01ly1ie5wsbr9zuj21kw1andu4",  // 逗号分隔，多为裸 pic id
    "video_url": "",                      // 实测恒空——视频实际在 url_info 里
    "location": "",
    "created_at": "$D2026-06-16T05:39:16.000Z",   // Next.js Date 标记，前缀 $D
    "source": "",
    "attitudes_count": 611, "comments_count": 54, "reposts_count": 50,  // 不抽取/不入库，仅留在 raw

    "retweet_id": "",                     // 非空=转发，值为被转发微博 id
    "url_info": "$1f"                     // flight 引用（$ref），解析后是数组：t.cn 短链展开 + 视频信息
  }
  ```

- **关键更正：`text` 是微博原始纯文本，不是 HTML**——含 `\n` 换行、内联的 `#话题#`
  与 `@用户`、全角空格。站点上看到的 `<br>`/链接/图片是前端从 `text` + 结构化字段
  渲染出来的。所以本源的渲染是 **纯文本 + 结构化字段 → Markdown**，而非 HTML→Markdown。
- **图片只出现在 `pics` 字段**，逗号分隔，每个条目是**两种形态之一**（实测 34 张里
  30 张裸 id、4 张完整 URL）：
  - **裸 pic id（多数，无扩展名）**：如 `53899d01ly1ie5wrym85ej20sg0gl41f`；需自行
    构造可用 sinaimg URL（见「图片解析与下载」）。
  - **完整 sinaimg URL（少数）**：如 `https://wx2.sinaimg.cn/large/xxx.jpg`，可直接用。
    SSR 渲染出的 `<article>` prose HTML 里**不含 `<img>`**（图片由前端侧渲染）。因此要
    转存图片到 OSS，**必须解析这段嵌入 JSON**，从 `<article>` 抓取拿不到图。
- **转发原文随同嵌入**：转发帖与其被转发原文对象**在同一页的 flight 负载里并存**
  （38 条样本中 9 条转发、原文 9/9 同页）。故内联转发原文常态下**无需额外请求**，
  只需在解析一页时建立 `id → post` 映射、就地解析 `retweet_id`。
- **视频与短链在 `url_info`，不在 `video_url`**：`video_url` 字段实测**恒空**；视频
  与正文里的 `t.cn` 短链统一落在 `url_info`——它是一个 `$ref`（如 `"$1f"`）指向另一条
  flight 行，解析后是数组，每项形如 `{"short_url":"http://t.cn/…","url_title":"…",
  "long_url":"…","url_type":N}`。视频项的 `url_title` 含"微博视频"、`long_url` 为
  `video.weibo.com/show?fid=…`。`url_type` 不可靠（39 既可能是"微博视频"也可能是
  "查看图片"），需结合 `url_title`/`long_url` 判定。
- flight 负载被切成多个 `self.__next_f.push([1,"…"])` 分片，**需按序拼接后再做
  花括号配平**抽取 `{"id":"<数字"…}` 对象（单分片内 brace-match 会因跨分片而断裂）。
- 该站自带 `/rss`（真 RSS，每页 20 条），但其 item 正文里的图片是
  `<img src="{裸 pic id}" alt="微博图片">`——**只有裸 id，没有主机/协议，不是可用的
  图片链接**（阅读器无法加载），转发图片同理（`alt="转发微博图片"`）。我们自建管线的
  价值正在于：把图片真正下载、转存自有 OSS，并在输出 RSS 中嵌入**可用的 OSS 链接**。

### 已确认的设计决策（与作者讨论结果）

1. **解析数据源**：解析页面嵌入的 RSC JSON（而非抓 `<article>` prose HTML）。
   正文取 `text`（纯文本 → Markdown），图片取 `pics`。抓取到的就是 HTML 页面，
   从中提取嵌入 JSON 即可（无需浏览器渲染）。
2. **数据库表**：**新建** `tombkeeper_*` 表，与镜像站数据模型对齐；不复用已休眠、
   基于官方 API 的 `weibo_*` 表（那套未接 cron/路由，字段也不匹配）。
3. **原始信息存储**：每条微博把其原始 RSC JSON 对象存入 `tombkeeper_post.raw`
   （`bytea`），与 zsxq/zhihu 的 `raw` 约定一致，满足需求「保存原始信息」。
4. **转发处理**：与 `tombkeeper.io` 一致，**内联展示被转发原文**。由于原文与转发帖
   同页嵌入，解析时就地从 `id → post` 映射取原文、把其 `text`/图片内联进转发帖的
   Markdown；并记录 `retweet_id`。极少数原文不在同页时回退请求一次
   `/weibo/{retweet_id}`，仍失败则只渲染转发帖自身 `text`。
5. **标题**：RSS item 标题用正文**截取**（取 `text` 首段/前 N 字，去换行）。
6. **图片下载失败**：需**落库记录失败**，避免后续抓取重复下载（详见数据库设计的
   `tombkeeper_object.status`）。
7. **ID 规范（决策）**：微博有数字 `id`（mid）与 `bid`（mblogid）两种标识。**以数字
   `id` 为全局规范主键与一切交叉引用键**（tombkeeper.io 详情、`weibo.com/detail`、
   列表 JSON 均用数字 id）。链接里出现的 `bid` 一律先经 **mblogid↔mid base62 算法**
   转成数字 id 再使用；`bid` 仅作普通列保留（回链/排查），不作查找键。
8. **多账号（硬编码）**：tombkeeper 有**两个微博账号**，均会出现在 tombkeeper.io——
   `1401527553`（tombkeeper）与 `6827625527`（t0mbkeeper）。数量恒定，**在一个 Go 服务里
   硬编码**这两个 uid 即可（后续再抽象成接口），不建 DB 表。「是否 tombkeeper 本人」=
   `author_id`/链接 `uid` 是否在该集合。
9. **归档页排在 PLAN 后期**：`rss_url` 归档页是 DB + 解析系统的**自然派生**，**写进 PLAN、但
   排在后期**（不是不写）；URL 形态先定下来供链接指向，handler 放到最后实现（不阻塞链接识别/内联）。
10. **标题列**：`tombkeeper_post` 设独立 `title` 列；v1 用正文**前 10 字**填充，后期用 **LLM**
    细化（写进 PLAN 后期）。

## 目标

1. 新增 `pkg/routers/tombkeeper` 源：抓取 `tombkeeper.io` 列表页 → 解析嵌入 JSON
   → 正文 HTML 转 Markdown（图片链接替换为 OSS）→ 落库 → 渲染 Atom RSS。
2. **硬编码每小时 cron**（`0 * * * *`），每次抓取 **2 页**（共约 10 条），按
   `id` 去重 upsert，已存在则跳过。
3. **图片转存 OSS**：下载 `pics` 里的 sinaimg 图片（带 `Referer` 绕过防盗链），
   上传到 OSS，正文 Markdown 中把原图链接替换为 OSS 链接。
4. 对外暴露 `/rss/tombkeeper`，返回 Atom RSS，结果缓存到 Redis（沿用 macked 形态）。
5. **保存原始信息**：每条原始 JSON 存 `raw`。
6. **`url_info` 短链处理 + 微博正文识别 + 隐式引用**（详见 SSOT）：把 `text` 里的
   `t.cn` 短链按 `url_info` 展开；识别指向 tombkeeper 自身微博（`uid` 在硬编码账号集合）
   的链接，行内替换为指向我方**归档页**的 Markdown 链接，并在文末以引用块内联被链接博文；
   其余链接展开为普通链接。识别时若需抓取被链接帖则顺手入库（即避免重复抓取）。
7. **新增单条微博归档页**（`rss_url`，类比 zhihu `/api/v1/archive`，PLAN 后期），供上述链接指向。

## 数据源与抓取

- 抓取 URL：`https://tombkeeper.io/?page=1` 与 `?page=2`（页码硬编码，每次 2 页）。
- HTTP：复用项目现有请求约定（自定义 User-Agent；限速/重试）。镜像站无需 cookie。

## 字段说明与解析规则（以 SSOT 为准）

**微博字段语义与解析/渲染规则（正文纯文本→Markdown、`pics` 多图与裸 id 并发探测下载、
`url_info` 短链展开/微博正文识别 + 流程图、转发内联、视频、标题、mid↔bid 算法）以
[`pkg/routers/tombkeeper/example/README.md`](../../pkg/routers/tombkeeper/example/README.md)
为单一事实源（SSOT），本 SPEC 不重复。** 该目录另含 6 个典型案例的原始 JSON 夹具（单图/多图/
微博正文链接/嵌套转发/视频）供构建可复现单测。

要点速览（细节与流程图见 SSOT）：

- 正文 `text` 是**纯文本**（非 HTML）→ Markdown；图片只在 `pics`、视频只在 `url_info`。
- `pics` 多为**裸 pic id**：参考 image-seeker 展开候选 CDN（sinaimg `wx/ww/tvax`+第三方代理）
  **并发探测**取首个可用；**所有 CDN 无响应即放弃该图**（`object.status=1`，正文**保留原始图片链接**）。
- `url_info` 短链：识别指向 tombkeeper 本人（`uid ∈` 硬编码账号集合）的「微博正文」链接，行内
  换 `[微博正文N](rss_url)` 并在文末引用块内联被链接帖（隐式引用，**只 1 层**）；其余展开为普通
  `[url_title](long_url)`。取被链接帖：`uid 判账号 → bid→mid(base62) → 查 DB → 抓站入库`（无检测表）。
- 转发 `retweet_id`：原文与转发帖同页嵌入，就地内联（**只 1 层**），缺失回退一次 `/weibo/{id}`。
  被转发原文 `pics` 的图片属于原文，渲染在**引用块内**。
- 「查看图片」（`url_type:39`）：是转发者**带图转发**时附加的**正文图片**（**不是**被转发原文的
  图片）；`pics` 为空，真图经 `long_url` 的 `photo.weibo.com` H5 页解析出 sinaimg 取得。渲染：正文
  内把该 `t.cn` 短链就地换成 `[微博图片 N]` 链接、并把该正文图片 `![微博图片 N]` 放到转发引用块
  **之前**（与原文图片区分；详见 SSOT）。

## 数据库设计（需新建）

### 表 `tombkeeper_post`

| 列              | 类型             | 说明                                                  |
| --------------- | ---------------- | ----------------------------------------------------- |
| `id`            | `bigint` PK      | 微博数字 id（mid，全局规范键，如 `5311127265215757`） |
| `bid`           | `text`           | mblogid 短 id（仅作回链/排查；查找一律走 id）         |
| `author_id`     | `text`           | user_id（属哪个 tombkeeper 账号，见账号集合）         |
| `screen_name`   | `text`           | 作者昵称                                              |
| `created_at`    | `timestamptz`    | 微博发布时间（建索引，分页/排序用）                   |
| `title`         | `text`           | RSS 标题（v1=正文前 10 字，后期 LLM 细化）            |
| `text_markdown` | `text`           | 正文 Markdown（图片→OSS、短链已处理、含内联引用）     |
| `video_url`     | `text`           | 视频链接（来自 `url_info`，可空）                     |
| `retweet_id`    | `text`           | 被转发微博 id（可空）                                 |
| `raw`           | `bytea`          | 原始 RSC JSON 对象（需求 #6）                         |
| `created_db_at` | `timestamptz`    | 入库时间（autoCreateTime）                            |
| `updated_db_at` | `timestamptz`    | 更新时间（autoUpdateTime）                            |
| `deleted_at`    | `gorm.DeletedAt` | 软删除                                                |

### 表 `tombkeeper_object`（仿 zsxq Object）

| 列                 | 类型             | 说明                                                |
| ------------------ | ---------------- | --------------------------------------------------- |
| `id`               | `text` PK        | sinaimg 文件名（pic id）                            |
| `post_id`          | `bigint`         | 所属微博 id（含被转发原文的图片）                   |
| `type`             | `int`            | 0 = image                                           |
| `object_key`       | `text`           | `tombkeeper/{picid}.jpg`（放弃时空）                |
| `url`              | `text`           | 原始 sinaimg URL                                    |
| `storage_provider` | `text[]`         | OSS assets domain                                   |
| `status`           | `int`            | 0 = 成功，1 = 放弃（所有 CDN 无响应，正文保留原链） |
| `created_at`       | `timestamptz`    | autoCreateTime                                      |
| `updated_at`       | `timestamptz`    | autoUpdateTime                                      |
| `deleted_at`       | `gorm.DeletedAt` | 软删除                                              |

- **放弃即记录、避免重复下载**：仅当**所有候选 CDN 都无响应**时插入一行 `status=1`、
  `object_key` 留空（放弃），但正文 Markdown 中**保留原始图片链接**（`![](原始 sinaimg URL)`）；
  后续抓取遇同一 `id` 已存在即跳过、不再重试。
- 成功转存的图片 `status=0` 并写入 `object_key`，正文引用 OSS 链接。

> 账号集合（`1401527553` / `6827625527`）**硬编码在 Go 服务里**，不建 DB 表（详见决策 #8）。
> 不设「链接检测结果表」：是否 tombkeeper 由 `uid` 直接判定，避免重复抓取由「抓到即入库
> `tombkeeper_post`」保证（详见 SSOT §3）。

两张表均通过 GORM `AutoMigrate` 注册（`internal/migrate/db.go`），随服务启动自动建表。

## Cron 与去重

- 在 `cmd/server/cron.go` 以**硬编码**方式注册 job：`name=tombkeeper_crawl`、
  `schedule="0 * * * *"`、`fn=tombkeeper.CrawlFunc(...)`，与现有 `macked_crawl`
  并列（同为 `AddJob` 的 fire-and-forget 形态）。
- 每次运行抓 2 页，遍历条目：`id` 已存在则跳过；新条目解析→（短链处理：识别微博正文
  链接、按需抓取入库被链接帖、内联引用）→转存图片→落库。因内联而新抓入库的帖也会进入
  后续去重（同 id 不再重复抓）。
- 抓取完成后重新渲染 RSS 并写入 Redis 缓存。

## 对外接口

### RSS

- 路由：`GET /rss/tombkeeper`（参考 macked 的 `/rss/macked` 注册方式）。
- Controller：`internal/controller/tombkeeper`，从 Redis 取缓存；缺失则用 DB 最新
  N 条重新渲染并回填缓存。
- Redis key：`tombkeeper_rss`（新增 `internal/redis` 常量）。
- 渲染：用 `github.com/gorilla/feeds` 生成 Atom；item 标题取 `tombkeeper_post.title`
  （v1=正文前 10 字、去换行；后期 LLM 细化），正文 Markdown 转 HTML 作为 content。
- **item `<link>` 指向微博原文的 uid/bid 永久链接 `https://weibo.com/{uid}/{bid}`**（如
  `weibo.com/1401527553/R5pVD1Ek5`，不是 detail/mid 形式、也不是镜像站）；正文**末尾**追加两个
  链接：`[存档链接](rss_url 归档页)` 指向我方自建归档、`[粉丝站链接](https://tombkeeper.io/weibo/{id})`
  指向 tombkeeper.io 镜像站。归档页同时支持 uid/bid 与 detail/mid 两种 URL 形态。
- **互动数不记录、不展示**：`attitudes_count` / `comments_count` / `reposts_count`
  不抽取为字段、不入库、不在 RSS 中展示（这些值仍原样保留在 `raw` 的原始 JSON 里）。

### 单条微博归档页（`rss_url`）—— PLAN 后期实现

为「微博正文链接」提供可点击的归档目标，类比 zhihu 的 `/api/v1/archive/:url`：给定一条
tombkeeper 微博（按数字 `id` 定位），渲染我方存的 Markdown（→HTML，图片为 OSS 链接）。
能渲染的前提是该帖在 `tombkeeper_post`（链接识别时已确保入库）。

**实现次序**：它是 DB + 解析/渲染系统搭好后的**自然派生**（本质就是"渲染单条已存帖"），
**写进 PLAN、但排在后期**（不是不写）。`rss_url` 的 **URL 形态现在就定下来**（供链接识别阶段
生成 `[微博正文N](rss_url)`），handler 后期补上即可——链接识别/文末内联不依赖此页存在。
路由形态（复用 `/api/v1/archive/:url` 还是新建按数字 id 的子路由）留 PLAN 定。

> 归档页 HTML **不显示标题**（`title` 列只用于 RSS item 标题）；正文末尾附
> `[微博] · [粉丝站]` 源链接（类比 zhihu archive 的「原文链接」）。

## 非目标

- 不接入 cookie / 官方微博 API；只消费 `tombkeeper.io` 镜像站。
- 转发原文仅就近内联（同页负载 / 最多回退一次请求），不做深层多级转发递归展开。
- 微博正文链接的内联同样**只 1 层**，被内联帖里的微博正文链接不再递归内联。
- 不做历史全量回填（6558 页）；只增量抓最新 2 页。本期不提供手动回填接口
  （如需要，留待后续 TODO）。
- `#话题#`、`@用户` 按纯文本渲染、不做斜体、不做链接化（`t.cn` 短链按上节展开/识别处理）。
- 不复用、不改动已休眠的 `weibo_*` 表与 `pkg/routers/weibo`。
- cron 周期与抓取页数按需求硬编码，不做成可配置项。

## 待 PLAN 阶段细化的点

1. 图片 CDN 候选清单与**并发探测**实现（照搬 image-seeker 的主机变体与 3rd-party 代理；
   并发取首个可用；扩展名默认 jpg / 按 `Content-Type` 取 gif）。
2. RSC flight 负载的稳健提取实现（按序拼接 `__next_f.push` 分片 → 花括号配平抽
   `{"id":"<数字"…}` + 解析 `url_info` 的 `$ref` / `$D`）与解析失败、字段缺失的容错。
3. 纯文本 → Markdown 的转义细则（需转义哪些字符、`\n` 与连续空行的处理、转发原文
   与微博正文链接内联引用块的具体排版）。
4. **微博正文链接子系统**：`bid ↔ mid` base62 算法落地与单测（用夹具真实 (mid,bid) 对、
   字母表小写在前）；识别时同步抓取被链接帖对单次 cron 请求量的影响与限速。
5. **PLAN 后期项**：`title` 的 LLM 细化；`rss_url` 归档页（路由形态：复用
   `/api/v1/archive/:url` 还是新建按数字 id 的子路由）。二者均写进 PLAN、排在后期。
