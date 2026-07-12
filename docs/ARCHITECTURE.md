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

见 [issues/2026-06-24-unified-rss-pipeline.md](issues/2026-06-24-unified-rss-pipeline.md) /
[plans/2026-06-24-unified-rss-pipeline.md](plans/2026-06-24-unified-rss-pipeline.md)。核心：
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

- **内容库**：Postgres（GORM）。多数来源保存已解析正文；tombkeeper 只保存结构化博文、链接、
  图片资产和时间线成员关系，读取时先批量装配 `ContentSnapshot`，再用纯函数生成 Markdown。
- **缓存**：Redis 存 `cachedFeed` JSON 与部分渲染 XML（random 端点 24h TTL）。
- **对象存储**：图片抓取后转存 OSS 换链（tombkeeper/zsxq 共用 `internal/file`）。
- **tkblog 博客（旁支，不入 RSS 管线）**：`tombkeeper.io/{xfocus,baidu}` 的博文另存
  `tombkeeper_blog_post`（`category` 区分两源、复合主键 `(category,id)`），纯文本正文存已转义
  markdown。**只做解析/落库 + 单篇归档 HTML，无 RSS 出口**；按需**全量**抓取（伪 job，无 cron，
  见 [OPS](OPS.md)）。解析复用微博同款 Next.js flight 机制（`pkg/routers/tkblog`）。

## 定时任务（cron）

`pkg/cron` 是 gocron 的薄封装；`cmd/server/cron.go` 的 `setupCronCrawlJob` 在启动时装配两类
job：

- **静态 job**：`jobDefinition` slice（`check_cookies` / `macked_crawl` / `tombkeeper_crawl` /
  `canglimo_*` / `zvideo_crawl` / `douyu_crawl`），固定注册，与来源枚举无关。
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
