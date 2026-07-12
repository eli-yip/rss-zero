---
title: "cron 来源注册表重构 + 删除 definitionToFunc 数据竞争"
issue: docs/issues/2026-07-12-cron-source-registry.md
status: done
areas: [cron, controller, db]
updated: "2026-07-12"
---

# PLAN: cron 来源注册表重构 + 删除 definitionToFunc 数据竞争

## 目标

解决 [issue](../issues/2026-07-12-cron-source-registry.md)：把散在两个文件的 5 处来源 `switch`
收敛到一张表驱动的来源注册表，让「新增一个来源」从「改 5 处」变成「加 1 行 + 写 1 个闭包」；
并借重构顺手删掉 `DefinitionToFunc` 共享 map，从构造上消除 `concurrent map read and map write`
数据竞争。不动数据库、不动静态 job、不引入接口抽象。

## 当前症结

来源枚举 `pkg/cron/db/definition.go:34` 起的 `iota` 块只有 4 个值，却被 5 处 switch 分别翻译成
「构造哪个 crawlFunc」「是否可续爬」「字符串 ↔ int」。其中两处 add-flow
（`cmd/server/cron.go:165` 与 `internal/controller/job/task.go:71`）几乎逐字重复。四个
`Build*` 签名彼此不兼容，但返回值统一是 `CrawlFunc = func(chan cron.CronJobInfo)`
（`internal/controller/job/controller.go:34`）——这正是「一个统一出口、四个不同构造」的表驱动
场景，`pkg/cookie/registry.go` 已用同样手法（`Spec` slice + `AllSpecs()`）处理过 cookie。

`DefinitionToFunc`（`controller.go:35`）这张无锁 map 是历史包袱：它唯一的用途是喂给
`StartJob`（`job.go:30`）一个 crawlFunc，但 `StartJob` 上一行（`job.go:21`）已经从 DB 取到完整
definition。三个并发 handler（add/patch 写 `task.go:110`、delete 写 `task.go:222`、start 读
`job.go:30`）无锁读写同一 map，同 taskID 并发即 panic 崩进程。

## 关键决策

### 1. 一张来源注册表，照抄 `pkg/cookie/registry.go` 的表驱动风格

**选择**：在 `internal/controller/job` 新增 `registry.go`，一源一行 `SourceSpec`，一个包级
`registry []SourceSpec` slice，配 `SpecByType` / `SpecByName` 两个线性查找 helper（来源只有 4 个，
线性扫描足够，不建 map）。

```go
type BuildDeps struct {
    Redis    redis.Redis
    Cookie   cookie.CookieIface
    DB       *gorm.DB
    AI       ai.AI
    Notifier notify.Notifier
}

// ResumeInfo 是跨源通用的续爬输入；不可续爬的源忽略它。
type ResumeInfo struct{ JobID, LastCrawled string }

type SourceSpec struct {
    Type      int    // 现有 cronDB.TypeXxx，注册表键（本 issue 不动）
    Name      string // "zsxq" 等，与 API 字符串一致
    Resumable bool   // 仅 zsxq/zhihu 为 true
    Build     func(deps BuildDeps, def *cronDB.CronTask, resume *ResumeInfo) CrawlFunc
}

func (s SourceSpec) JobName() string { return s.Name + "_crawl" }
```

**理由**：四个 `BuildCrawlFunc` 签名各异（zsxq/zhihu 全参数含 resume/ai、xiaobot 用 `Filter`
结构无 ai/resume、github 只有 redis/cookie/db/ai/notifier），无法收敛成统一函数签名。表驱动的
惯用解法就是把「差异」关进闭包：每行的 `Build` 闭包内部各自按本源的真实签名调用
`zsxqCron.BuildCrawlFunc` / `zhihuCron.BuildCrawlFunc` / `xiaobotCron.BuildCronCrawlFunc` /
`githubCron.Crawl`，对外只暴露一个统一入口 `func(BuildDeps, *CronTask, *ResumeInfo) CrawlFunc`。
`JobName()` 复现现有的 `"<name>_crawl"` 命名（`AddCrawlJob("zsxq_crawl", …)` 等），API 字符串
就是 `Name`，`TypeStrToInt/IntToStr` 也随之退化成查表。这与 cookie 注册表的
`AllSpecs()` / `SpecByType()` / `Label()`（`pkg/cookie/registry.go:96,106,109`）一一对应。

`BuildDeps` 只打包四个源实际用到的依赖（redis/cookie/db/ai/notifier）；没有源需要 fileService，
故不放进去。`ResumeInfo{JobID, LastCrawled}` 是通用结构，可续爬的两个源在各自 `Build` 闭包里把它
映射成源专属的 `zsxqCron.ResumeJobInfo` / `zhihuCron.ResumeJobInfo`（两者字段都是
`JobID, LastCrawled string`，`pkg/routers/{zsxq,zhihu}/cron/crawl.go:31`）。

注册表（示意，一源一行）：

```go
var registry = []SourceSpec{
    {Type: cronDB.TypeZsxq,    Name: "zsxq",    Resumable: true,  Build: buildZsxq},
    {Type: cronDB.TypeZhihu,   Name: "zhihu",   Resumable: true,  Build: buildZhihu},
    {Type: cronDB.TypeXiaobot, Name: "xiaobot", Resumable: false, Build: buildXiaobot},
    {Type: cronDB.TypeGitHub,  Name: "github",  Resumable: false, Build: buildGitHub},
}
```

### 2. 两处 add-flow 收敛到一个共享 helper

**选择**：`cmd/server/cron.go:165` 的 `addJobToCronService`（遍历所有 definition）与
`internal/controller/job/task.go:71` 的 `addTaskToCronService`（单条请求）改为都调用同一个
helper：

```go
func AddToScheduler(cronService *cron.CronService, cronDBService cronDB.DB,
    spec SourceSpec, deps BuildDeps, def *cronDB.CronTask) (jobID string, err error)
```

它做三件事：`spec.Build(deps, def, nil)` → `cronService.AddCrawlJob(spec.JobName(), def.CronExpr, fn)`
→ `cronDBService.PatchDefinition(def.ID, nil, nil, nil, &jobID)`，与现状逐字等价。

**理由**：这两段现在是复制粘贴（连日志文案都近乎一致），是「改一处忘另一处」的经典绊线。塌成一个
helper 后，启动期 `addJobToCronService` 变成「遍历 definitions → 查表 → 调 helper」；请求期
`addTaskToCronService` 变成「查表 → 调 helper」。两处的 default panic / 未知类型错误分支也随查表
失败统一成一条 `spec not found` 错误。`addTaskToCronService` 现有入参是散的
（taskID/cronExpr/include/exclude/taskType）；`AddTask` 里在 `AddDefinition` 之后先
`GetDefinition` 取回 `*CronTask` 再传给 helper（`PatchTask` 本就已持有 `taskInfo`），使 helper
只认 `*CronTask` 一种形态。

**可见性**：两个启动函数 `resumeRunningJobs` / `addJobToCronService` 按原决策**留在 `package main`**
（`cmd/server/cron.go:115,153`），它们跨包调用注册表，因此 `AddToScheduler` / `SpecByType` /
`SpecByName`（连同已大写的 `SourceSpec` / `BuildDeps` / `ResumeInfo`）必须**导出**。这与样板
`pkg/cookie/registry.go` 一致——`cmd/server/cron.go:217` 的 `checkCookies` 正是跨包调用导出的
`cookie.AllSpecs()` / `SpecByType()`。包内自用的 `buildDeps()`、四个 `buildXxx` 闭包保持未导出。

### 3. `resumeRunningJobs` 用 `Resumable` 标志分流，去掉 default panic

**选择**：`cmd/server/cron.go:131` 的 switch 改为查表后按 `spec.Resumable` 分流：

```go
spec, ok := SpecByType(definition.Type)
if !ok { return fmt.Errorf("unknown cron task type %d", definition.Type) }
if spec.Resumable {
    fn := spec.Build(deps, definition, &ResumeInfo{JobID: job.ID, LastCrawled: job.Detail})
    go cron.GenerateRealCrawlFunc(fn)()
} else {
    if err = cronDBService.UpdateStatus(job.ID, cronDB.StatusStopped); err != nil { ... }
}
```

**理由**：现状里 zsxq/zhihu 两个 case 除了包名之外完全一样（都是 build→`GenerateRealCrawlFunc`
→`go`），xiaobot/github 两个 case 也一样（都只 `UpdateStatus(StatusStopped)`）。这本质是一个布尔
维度，用 `Resumable` 表达比四个 case 精确。`job.Detail` 存的就是续爬点（`pkg/cron/db/job.go:19`
注释：zhihu 存 last crawled sub id），`job.ID` 是运行中 job 的 id，二者组成 `ResumeInfo`。

### 4. 删掉 `DefinitionToFunc`，`StartJob` 现场重建

**选择**：删除 `DefinitionToFunc` 类型（`controller.go:35`）、`Controller.definitionToFunc` 字段
（`controller.go:23`）、`addTaskToCronService` 里的 `h.definitionToFunc[taskID] = crawlFunc`
（`task.go:110`）、`DeleteTask` 里的 `delete(h.definitionToFunc, taskID)`（`task.go:222`），以及
它在 `setupCronCrawlJob` 返回值（`cmd/server/cron.go:31`）、`addJobToCronService` 返回值
（`cron.go:153/159/209/211`）、`cmd/server/main.go:83/86`、`cmd/server/echo.go:57` `setupEcho`
形参、`NewController` 形参与调用（`controller.go:27`、`echo.go:115-117`）里的全部线缆。
`StartJob`（`job.go:16`）改为：

```go
definition, err := h.cronDBService.GetDefinition(taskID) // 现状已有，job.go:21
spec, ok := SpecByType(definition.Type)
if !ok { return httputil.NewHTTPError(http.StatusBadRequest, "unknown task type") }
crawlFunc := spec.Build(h.buildDeps(), definition, nil) // Controller 已持有全部 deps
```

**理由**：`definitionToFunc` 是「运行态被当作长寿命共享状态缓存」的反模式，且这份缓存是无锁并发
读写，会 `fatal error: concurrent map read and map write` 崩进程。`StartJob` 本就先查了 DB
definition，crawlFunc 是可由 definition + Controller 依赖纯构造出来的东西，没有任何理由跨请求缓存。
现场重建后：add/patch/delete 不再写 map、start 不再读 map，**数据竞争由「不再有共享可变状态」这个
构造事实消除，而不是靠加锁**。Controller 已持有 redis/cookie/db/ai/notifier（`controller.go:16-24`），
补一个私有 `buildDeps()` 把它们打成 `BuildDeps` 即可，不新增依赖。

**一处有意的行为改善**：现状 `StartJob` 在 map 无此 entry 时返回 404（`job.go:31-33`
`"task definition not found"`）。改成现场重建后，只要 DB 里有 definition 就能启动；「DB 有
definition、但旧 map 恰好没这条」的边角从「404」变为「可正常启动」。这是有意为之的改善（那个 404
本就是缓存缺失的伪症状，不是真的找不到任务），实现评审不应把它当回归。仅当 `SpecByType` 查不到
（未知来源类型）才返回错误。

### 5. 明确不做

- **不引入 `Manager` / `Driver` 接口**：来源只有一种「构造 crawlFunc」的行为，且已有统一出口类型
  `CrawlFunc`；单实现的接口是纯仪式，用闭包表即可（YAGNI）。
- **不做 `init()` 自注册 / 插件化**：一个包内的静态 slice 显式列出 4 个源，比隐式注册更好读、更好
  测；来源不会运行期动态增减。
- **不动静态 job**：`cmd/server/cron.go:54` 的 `jobDefinition` slice（check_cookies / macked /
  tombkeeper / canglimo_* / zvideo / douyu）不吃来源枚举，保持原样。
- **不动数据库**：`Type` 仍是 int、`CronServiceJobID` 仍持久化。删列 / int→string 分别是
  [Issue 2](../issues/2026-07-12-cron-drop-jobid-column.md)、
  [Issue 3](../issues/2026-07-12-cron-kind-string.md)，都 `depends_on` 本 issue。

## 代码落点

- `internal/controller/job/registry.go`（新增）：`SourceSpec` / `BuildDeps` / `ResumeInfo`、
  `registry` slice、`SpecByType` / `SpecByName`、四个 `buildXxx` 闭包、`JobName()`、
  `TypeStrToInt` / `TypeIntToStr`（改为查表）。
- `internal/controller/job/controller.go`：删 `DefinitionToFunc` 类型与 `definitionToFunc` 字段，
  `NewController` 去掉该形参，补 `buildDeps()`。
- `internal/controller/job/job.go`：`StartJob` 改为查表现场重建。
- `internal/controller/job/task.go`：`addTaskToCronService` 改调共享 helper 并去掉 map 写；
  `DeleteTask` 去掉 map delete；`TaskTypeStrToInt` / `TaskTypeIntToStr` 删掉（迁移到 registry.go
  的查表实现，或就地改为调注册表函数）。
- `cmd/server/cron.go`：`resumeRunningJobs` 改 `Resumable` 分流；`addJobToCronService` 改遍历 +
  共享 helper，去掉 `DefinitionToFunc` 返回；`setupCronCrawlJob` 去掉该返回值。
- `cmd/server/main.go` / `cmd/server/echo.go`：删 `definitionToFunc` 变量与 `setupEcho` 形参传递。

## 实施步骤（对应提交）

1. **落注册表**：新增 `registry.go`（`SourceSpec` 表、四个 `Build` 闭包、`SpecByType/SpecByName`、
   `JobName`、Str/Int 查表），先写注册表自身单测。此步不改调用方，纯新增。
2. **收敛 add-flow**：抽出共享 `AddToScheduler` helper，`addJobToCronService` 与
   `addTaskToCronService` 改为查表 + 调 helper；`resumeRunningJobs` 改 `Resumable` 分流。Str/Int
   转换切到注册表。此步后 5 处 switch 全消失，但 map 仍在。
3. **删 map、消竞争**：删 `DefinitionToFunc` 及全部线缆，`StartJob` 改现场重建。补 `-race` 并发
   回归测试（在第 2 步基线上跑应 race，此步后转绿）。
4. **文档 + 评审**：更新 ARCHITECTURE / PROGRESS，`go build ./...`、`just lint`，跑 `/code-review`。

## 测试（必填）

> **测试基建现状**：`internal/controller/job` 包**当前零测试、无现成 harness**，本 plan 需自带
> test 供给。两块依赖都好造：`cronDB.DB` 是接口（`pkg/cron/db/db.go:7`，`CronTaskIface` +
> `CronJobIface`），喂一个**内存 fake**即可，这是本区既定路径；`cron.CronService` 用
> `cron.NewCronService(logger)` 纯内存构造（`pkg/cron/cron.go:18`，gocron 无外部依赖）。

- **注册表表本身**（`internal/controller/job`，纯内存、无 DB）：
  - `SpecByType` / `SpecByName` 双向往返：4 个来源都能按 int 查到、按 name 查到，且回环一致；
  - `JobName()` 对四源分别得 `zsxq_crawl` / `zhihu_crawl` / `xiaobot_crawl` / `github_crawl`；
  - `Resumable` 标志：仅 zsxq/zhihu 为 true；
  - `TypeStrToInt`("github")↔`TypeIntToStr`(TypeGitHub) 等四源往返；未知输入返回 error。

- **数据竞争回归（主，`go test -race`）——只并发写侧，绕开 `StartJob`**：
  对同一 taskID 起若干 goroutine 循环打 **`AddTask` / `PatchTask` / `DeleteTask` 三者**。这三个都是
  对 `definitionToFunc` 的 map 写/删（`task.go:110` 写、`task.go:222` `delete`），无锁 map 的
  write/write、write/delete 本身就会被 `-race` 判为竞争，并发够密时甚至直接 fatal
  `concurrent map writes`——**完全不碰 `StartJob`**，因此不会触发真实爬取。
  **不并发 `StartJob`**：`StartJob`（`job.go:37`）会 `go crawlFunc(...)` 真跑爬虫并 `select`
  阻塞至多 30s（`job.go:46`）；终态下经 `spec.Build` 拿到的是真实 zsxq/zhihu 爬虫闭包，「并发打
  StartJob」会真发抓取或挂满超时，测试里不可取。
  **红→绿证据**：这条测试针对 step 2 结束、「map 尚在」的那个基线提交跑一次 `-race`，取到「红」
  （race 报告 / fatal）作为竞争真实存在的证据；step 3 删 map 后同一测试转「绿」。这就是「修前必挂、
  修后转绿」的回归。

- **`StartJob` 读侧（可选，重赋 `registry` seam）**：若要覆盖 `StartJob` 现场重建这条读路径，
  在 `package job` 的测试里**把包级 `registry` 变量重赋为 no-op `Build` 闭包**——闭包只往传入的
  `chan cron.CronJobInfo` 塞一个成功的 `CronJobInfo` 就返回，不做任何抓取。点名这个 seam：
  `registry` 是包级 slice，测试可在 setup 里覆写、teardown 还原。这样能断言「DB 有 definition →
  `StartJob` 成功启动」以及决策 #4 里「map 缺 entry 不再 404」的行为，且不引入真实爬虫。

- **add / patch / delete / start 端到端**（同样用内存 fake `cronDB.DB` + no-op `registry`）：
  走一遍四类操作，断言调度器里 job 的增删与 definition 回写（`CronServiceJobID`）行为与重构前
  一致；`StartJob` 经 no-op spec 现场重建能拿到可用 crawlFunc 并返回成功。

- 验证命令：`go test -race ./internal/controller/job/...`、`go test ./pkg/cron/...`、
  `go build ./...`、`just lint`；不跑无关全量测试。

## 待更新文档

- [ ] `docs/ARCHITECTURE.md`：cron 段把来源注册表写成「来源的唯一分支点」，说明静态 job 与动态
      来源 job 的分工。
- [ ] `docs/PROGRESS.md`：完成 / 合并结论。
- [ ] `docs/TODO.md`：登记本次派生的 Issue 2（删 `cron_service_job_id` 列）、Issue 3（Type
      int→string），若已单独开 issue 则在 TODO 里只留指针。
- [ ] 本 issue：合并时 `status: closed`。

## 后续项

- [Issue 2](../issues/2026-07-12-cron-drop-jobid-column.md)：删持久化的 `cron_service_job_id` 列。
- [Issue 3](../issues/2026-07-12-cron-kind-string.md)：`CronTask.Type` int(iota) 改字符串 `Kind`，
  注册表直接以 `Kind` 为键。
- 二者都 `depends_on` 本 issue，plan 待本 issue 落地后再写。
