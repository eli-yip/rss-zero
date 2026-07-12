---
title: "cron 来源注册表：把 5 处来源 switch 塌成一张表，顺带根治 definitionToFunc 数据竞争"
kind: refactor
status: open
priority: high
areas: [cron, controller, db]
plan: docs/plans/2026-07-12-cron-source-registry.md
follow_ups:
  - docs/issues/2026-07-12-cron-drop-jobid-column.md
  - docs/issues/2026-07-12-cron-kind-string.md
related:
  - cmd/server/cron.go
  - internal/controller/job/task.go
  - internal/controller/job/controller.go
  - internal/controller/job/job.go
  - pkg/cron/db/definition.go
updated: "2026-07-12"
---

## 问题

cron 的**动态**爬虫任务（用户在 API 里增删的 zsxq/zhihu/xiaobot/github 抓取）注册在一个 4 值
来源枚举上（`TypeZsxq / TypeZhihu / TypeXiaobot / TypeGitHub`，`pkg/cron/db/definition.go:34`
起的 `iota` 常量块）。围绕这个枚举，代码里散着 **5 处 `switch`，分布在两个文件**：

1. `cmd/server/cron.go:131` `resumeRunningJobs` —— 重启后恢复运行中的任务：zsxq/zhihu 重建
   crawlFunc 并 `go` 起来续爬；xiaobot/github 只把状态标成 `StatusStopped`；有 default panic 分支。
2. `cmd/server/cron.go:165` `addJobToCronService` —— 启动期遍历所有 definition，`Build*` 出
   crawlFunc → `AddCrawlJob` → `PatchDefinition` 回写调度器 job id。
3. `internal/controller/job/task.go:71` `addTaskToCronService` —— 处理 `AddTask` / `PatchTask`
   请求，逻辑与 #2 **几乎逐字重复**（同样的 build → AddCrawlJob → PatchDefinition）。
4. `internal/controller/job/task.go:296` `TaskTypeStrToInt` —— API 字符串（"zsxq" 等）转 int。
5. `internal/controller/job/task.go:311` `TaskTypeIntToStr` —— int 转回 API 字符串。

**新增一个来源，要同时改这 5 处。** 四个 `BuildCrawlFunc` 的签名各不相同
（zsxq/zhihu：`resume + taskID + include/exclude + ai/notifier`；xiaobot：`Filter` 结构、无
ai/无 resume；github：连 include/exclude/resume 都没有），但它们最终都返回
`func(chan cron.CronJobInfo)`，即 `internal/controller/job` 里的 `CrawlFunc`
（`internal/controller/job/controller.go:34`）。只有 zsxq/zhihu 支持续爬。这份「一处枚举、
五处 switch、签名各异但出口统一」的形态，正是 `pkg/cookie/registry.go` 已经用表驱动
（`AllSpecs()`）解决过的同一类问题。

**顺带要根治的一个数据竞争。** `internal/controller/job` 维护一张无锁共享 map
`DefinitionToFunc = map[string]CrawlFunc`（`internal/controller/job/controller.go:35`，
控制器字段在 `controller.go:23`）。它被：`AddTask/PatchTask` 经 `addTaskToCronService`
写（`task.go:110`）、`DeleteTask` 写（`task.go:222` 的 `delete`）、`StartJob` 读
（`internal/controller/job/job.go:30`）。这三条都是 Echo 的并发 handler，**无任何锁**，
并发命中同一 taskID 时会触发 Go 运行时的 `fatal error: concurrent map read and map write`
（直接 panic，进程崩溃）。而这张 map 存在的唯一理由，是让 `StartJob` 拿到 crawlFunc ——
但 `StartJob` 在 `job.go:21` 已经从 DB 取到了完整 definition，`job.go:30` 只是拿它的 ID 去 map
里换回一个函数。有了注册表后，crawlFunc 可以由 definition 现场重建，这张 map 可以整个删掉，
**数据竞争由构造消除、无需加锁**。

## 目标

- 把上面 5 处来源 switch 全部塌成对一张来源注册表的查询；新增一个来源只改一行表 + 写它的
  `Build` 闭包。
- 删除 `DefinitionToFunc` 共享 map 及其全部线缆，让 `StartJob` 经注册表用 DB definition 现场
  重建 crawlFunc，从而消除 `concurrent map read and map write` 数据竞争。
- 注册表照抄 `pkg/cookie/registry.go` 的表驱动风格；四个不一致的 `BuildCrawlFunc` 签名各自锁进
  各源的 `Build` 闭包，对外只暴露统一入口。
- 行为不变：动态任务的增/删/改/启动、重启恢复（含 zsxq/zhihu 续爬、xiaobot/github 标 stopped）
  与现状逐一等价。

## 验收

- `internal/controller/job/registry.go` 新增，一源一行 `SourceSpec`；`resumeRunningJobs`、
  两处 add-flow、Str/Int 转换全部改成注册表查询，无来源相关的 `switch` 残留。
- 两处重复的 add-flow（`cmd/server/cron.go` 与 `internal/controller/job/task.go`）收敛到一个共享
  helper。
- `DefinitionToFunc` 类型、`Controller.definitionToFunc` 字段、`setupCronCrawlJob` /
  `NewController` / `setupEcho` / `main.go` 里对它的传递全部删除；`StartJob` 改为经注册表重建。
- 有一个 `go test -race` 并发回归测试：对同一 taskID 并发打 `AddTask/PatchTask/DeleteTask/StartJob`；
  在删 map 之前的基线上会 race/崩，改后转绿。
- 注册表往返测试通过（`specByType`/`specByName` 双向、`JobName()`、`Resumable` 标志正确）；
  add/patch/delete/start 端到端仍工作。
- `docs/ARCHITECTURE.md` 的 cron 段把注册表写成来源的唯一分支点。

## 不做什么

- 不引入 `Manager` / `Driver` 之类接口：单实现就写具体代码（YAGNI）；来源差异用闭包表承载，不用
  接口多态。
- 不做 `init()` 自注册 / 插件系统：一个包内的静态 slice 就够，显式优于隐式。
- 不动**静态** job（`check_cookies` / `macked_crawl` / `tombkeeper_crawl` / `douyu_crawl` 等，
  `cmd/server/cron.go:54` 起的 `jobDefinition` slice）：它们不吃来源枚举，保持原样。
- 本 issue **不动数据库**：`CronTask.Type` 仍是 int（`definition.go:17`）、`CronServiceJobID` 列
  （`definition.go:21`）仍保留。删列是 [Issue 2](2026-07-12-cron-drop-jobid-column.md)、int→string
  是 [Issue 3](2026-07-12-cron-kind-string.md)，都依赖本 issue 先落地。
- 不改 `pkg/cron`（gocron 封装）与各源 `BuildCrawlFunc` 的内部实现，只在注册表里包住它们。
