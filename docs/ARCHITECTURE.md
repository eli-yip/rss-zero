# 架构

RSS-ZERO 后端：把若干「私有/不友好」站点抓取、入库、统一渲染成 Atom 输出。纯 Go，无
headless 浏览器。本文件是代码地图 —— 模块边界、数据流、关键决策的落点，供人和 agent 快速定位。

## 全景

```
cmd/server        Echo HTTP 服务（:8080）—— /rss/<source>、/api/v1/*（含 /api/v1/health）
cmd/cli           运维/一次性任务 CLI

internal/         应用内部（不对外复用）
  controller/     各源的 HTTP handler + 编排（zhihu xiaobot github zsxq tombkeeper
                  endoflife macked …，另有 archive job migrate parse rsshub user cookie）
  weibochat/      微博群聊存档核心（抓取、纯解析、Postgres 存储和附件转存）
  rss/            统一 RSS 出口管线：canonical Item + FeedMeta + RenderAtom + 缓存层
  migrate/        迁移注册表（schema_migrations 表，启动自动跑）
  db/ redis/ file/ 存储访问（Postgres/GORM、Redis、对象存储/OSS）
  md/ notify/ ai/ log/ middleware/ version/ utils/  markdown、Bark、AI、日志等

pkg/              可复用/源特定
  routers/<src>/  各源的抓取 + 解析 + （旧）渲染：zhihu xiaobot github zsxq
                  tombkeeper tkblog endoflife macked weibo douyu
  render/         共享 markdown/HTML/Atom 渲染 helper（goldmark 封装）
  cookie/ cron/ httputil/ bookmark/ embedding/ common/
```

## RSS 出口管线（统一收口）

`/rss/<source>` 这一层统一为一条管线，取代早期「6+ 套 render 结构体 + 各写各的 Atom」。

- **canonical 类型**：`Item{ID,Link,Title,Author,Time,Summary,ContentHTML}` + `FeedMeta`，
  唯一渲染器 `rss.RenderAtom`。各源 `Fetch` 把 `ContentHTML` 算好，渲染器不再加工。
- **缓存下沉**：从「渲染后的 XML」下沉到 `cachedFeed{Meta,Items}` 的 JSON（`v2:` key 与旧
  XML 隔离）；`MaxFetch=50`，limit 不进 key、按需切片。
- **Fetch 归属**：`zhihu/xiaobot/github/zsxq` 在 `internal/rss`；`endoflife/tombkeeper/macked`
  在各自源包（非导出类型 + 规避 import 环）。
- **编排**：`rss.Serve` / `WarmCache` / `FetchCached`；DB 源的 cron 预热同一 key。
- **Markdown**：统一 goldmark `GFM + NewCJK(CSS3Draft)`。

## 数据流

```
cron / 请求 → controller.<source> → routers.<source>.Fetch（抓取+解析）
   → 入库(Postgres/GORM) + 图片转存(OSS) → internal/rss（canonical Item + 缓存 Redis）
   → RenderAtom → /rss/<source> 响应
```

- **内容库**：Postgres（GORM）。部分来源保存已解析正文；tombkeeper、zhihu、zsxq 只存结构化事实
  （`raw` + 侧表：object/question/article/author），读取时先批量装配 `ContentSnapshot`，再用纯
  `RenderMarkdown` 生成 Markdown。
- **缓存**：Redis 存 `cachedFeed` JSON 与部分渲染 XML（random 端点 24h TTL）。
- **对象存储**：图片抓取后转存 OSS 换链（tombkeeper/zsxq 共用 `internal/file`）。
  tombkeeper 在下载候选胜出前使用标准库 `http.DetectContentType` 检查真实内容，接受 JPEG/PNG/GIF/WebP，
  拒绝 HTML 等非图片响应；该步骤验证类型，不做完整像素解码。
- **tkblog 博客（旁支，不入 RSS 管线）**：`tombkeeper.io/{xfocus,baidu}` 的博文另存
  `tombkeeper_blog_post`（`category` 区分两源、复合主键 `(category,id)`），纯文本正文存已转义
  markdown。**只做解析/落库 + 单篇归档 HTML，无 RSS 出口**；按需**全量**抓取（伪 job，无 cron，
  见 [OPS](OPS.md)）。解析复用微博同款 Next.js flight 机制（`pkg/routers/tkblog`）。

## 微博群聊存档（独立核心）

`internal/weibochat` 不接入 RSS 管线。消息入口为 `Archiver.Initialize / Sync / Backfill`，附件入口为
`AssetArchiver.ArchiveAssets`，调用示例见 [WEIBO_CHAT.md](WEIBO_CHAT.md)。

- `client.go` 只处理 HTTP 协议，`parse.go` 为纯解析，`store.go` 封装 GORM 幂等写入；`sync.go` 编排消息，`assets.go` 编排下载和流式上传。
- 四张表由现有 AutoMigrate 建立：`weibo_chat_message`、`weibo_chat_sync_state`、`weibo_chat_asset`、`weibo_chat_user`。消息主键为 `(group_id, message_id)`，资产主键为 `(group_id, asset_key)`。
- `weibo_chat_user` 以 UID 为主键，保存最近观察到的昵称、头像原地址及转存结果；消息通过 `sender_id` 关联最新用户资料。每页消息先落库，再同步整页用户，包括已经存档的消息。头像转存成功后更新来源与存储位置，失败返回错误且不推进水位，下一次重试。
- 消息 `content.assets` 保存有序的资源引用；`asset_key` 使用 `file/{fid}`、`pic/{pid}`，只有 URL 的资源使用 `url/{URL 的 SHA-256}`。不建立附件关系表。
- 写入消息和附件引用成功后才推进游标，不建立跨网络或整轮数据库事务。重跑时读取首次存档内容补齐缺失资产，不覆盖旧消息或成功的附件。
- `synced_through_message_id` 是增量水位，`backfill_before_message_id` 是历史排他游标；读取全部原始消息计算边界，不过滤系统消息。
- 头像沿用现有 bucket，路径为 `weibo-chat/users/{uid}/{源 URL 的 SHA-256}.{ext}`，用户表保留来源地址、对象 key 和资源域名。
- 附件沿用现有 bucket 和资源域名，路径为 `weibo-chat/{group_id}/{asset_key}.{ext}`；扩展名由实际内容类型决定。上传成功后记录存储结果，失败可重试。

## 定时任务（cron）

`pkg/cron` 是 gocron 的薄封装；`cmd/server/cron.go` 的 `setupCronCrawlJob` 在启动时装配两类
job：

- **静态 job**：`jobDefinition` slice（`check_cookies` / `macked_crawl` / `tombkeeper_crawl` /
  `canglimo_*` / `zvideo_crawl` / `douyu_crawl`），固定注册，与来源枚举无关。
- **tombkeeper 告警边界**：live/history 都在一次 run 的最外层解释结果；panic、fatal error、成功但含
  可恢复单条失败三种结果互斥，每次 run 最多发一条聚合 Bark。单条失败继续处理，摘要保留总数与至多
  3 条代表性错误；手工 run-now 复用同一个 live cron 闭包。
- **动态来源 job**：用户经 `/api/v1/job` 增删的 zsxq/zhihu/xiaobot/github 抓取任务，持久化在
  `cron_tasks`（`CronTask.Type` 为 int 枚举）。

**来源的唯一分支点是 `internal/controller/job/registry.go` 的一张 `SourceSpec` 表**：一源一行
（`Type` / `Name` / `Resumable` / `Build` 闭包）。四个签名各异的 `BuildCrawlFunc` 各自锁进本源的
`Build` 闭包里，对外统一出 `CrawlFunc`；`SpecByType` / `SpecByName` 查表取代了原先散在「启动加载 /
请求增改 / 重启恢复 / 字符串↔int」的 5 处 `switch`。启动期与请求期共用 helper `AddToScheduler`
（`Build → AddCrawlJob → PatchDefinition` 回写调度器 job id）。重启恢复 `resumeRunningJobs` 按
`spec.Resumable` 分流：可续爬的 zsxq/zhihu 现场重建 crawlFunc 续跑，xiaobot/github 标
`StatusStopped`。`StartJob` 也由 definition + 注册表**现场重建** crawlFunc（无共享缓存 map，故无并发
数据竞争）。**新增一个来源 = 加一行表 + 写它的 `Build` 闭包**，别处不再改。

## 迁移

`internal/migrate` 是注册表：`Migration{Version int64, Name, Auto, RequiresPredecessors, Run}`
配 `schema_migrations` 集合表。启动时 `RunAuto` 按版本升序跑合格的 auto 迁移（前置门控、失败
只记日志/Bark 不阻断、下次启动重试）。手动端点：`registry` / `run/:version` / `run-pending`。
多数为「离线回填已存正文」的数据迁移（幂等）。

## 配套服务

- **rss-zhihu-encrypt**（`../../zhihu-encrypt`）：知乎加密服务，compose 内 `:3000`。
- **webapp**（`../webapp`）：前端，与本后端配对发布。
- 详见 [OPS.md](OPS.md)。
