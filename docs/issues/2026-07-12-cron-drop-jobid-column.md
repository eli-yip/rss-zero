---
title: "删除持久化的 cron_service_job_id 列，改由内存维护 taskID→schedulerJobID"
kind: tech-debt
status: open
priority: low
areas: [cron, db, controller, migrate]
plan: docs/plans/2026-07-12-cron-drop-jobid-column.md  # TBD，待 Issue 1 落地后补写
depends_on:
  - docs/issues/2026-07-12-cron-source-registry.md
related:
  - pkg/cron/db/definition.go
  - internal/controller/job/task.go
  - cmd/server/cron.go
updated: "2026-07-12"
---

> **plan 待写**：本 issue 依赖 [Issue 1（来源注册表）](2026-07-12-cron-source-registry.md) 先
> 落地。`plan:` 字段指向的 `docs/plans/2026-07-12-cron-drop-jobid-column.md` 目前是占位路径，
> 等 Issue 1 合并后再补写方案。

## 问题

`cron_tasks.cron_service_job_id`（`pkg/cron/db/definition.go:21`）持久化的是 **gocron 调度器进程内
的 UUID**：`AddCrawlJob` 每次 `s.NewJob` 都会由 gocron 生成一个新 UUID（`pkg/cron/cron.go:28-39`
返回 `j.ID().String()`）。而每次进程启动，`addJobToCronService`（`cmd/server/cron.go:165` 一带）
都会对所有 definition 重新 `AddCrawlJob`（拿到**新** UUID）并 `PatchDefinition` 覆盖旧值。

也就是说：**持久化的这个值跨重启毫无意义** —— 上次进程写进 DB 的 UUID，在下次进程里既不存在于新
调度器、也会被启动流程立刻覆盖。它是纯技术债：一个「运行态」被错误地当成「业务态」持久化。

这个列在进程内的唯一消费点，是把 job 从调度器摘掉时传给 `RemoveCrawlJob`：

- `PatchTask` 改任务时移除旧 job（`internal/controller/job/task.go:171`）；
- `DeleteTask` 删任务时移除 job（`internal/controller/job/task.go:210`）。

两处读的都是刚从 DB `GetDefinition` 拿到的行里的 `CronServiceJobID`。

## 目标

不再持久化调度器 job id；删掉 `cron_service_job_id` 列；改由内存 `map[taskID]schedulerJobID`
维护「definition → 当前调度器 job」的映射，`AddCrawlJob` 后写入、`RemoveCrawlJob` 前读取。

## 验收

- `cron_tasks` 表不再有 `cron_service_job_id` 列；有一条自动 migration 删列（`DROP COLUMN
  IF EXISTS`）。
- `CronTask` 结构体去掉 `CronServiceJobID` 字段及其在 `PatchDefinition` 第五参
  （`definition.go:52,68-70`）里的写入。
- `PatchTask` / `DeleteTask` 改从内存 map 取 schedulerJobID 摘 job；映射在 add-flow 写入、
  delete 后清理。
- 内存 map 有锁，且有 `go test -race` 并发测试覆盖 add/patch/delete 同 taskID。

## 不做什么 / 已知代价（必须落在文档里）

**这是一次「把无害但没用的 DB 列，换成一张必须加锁的内存 map」的搬家，并发面并没有减少。**
[Issue 1](2026-07-12-cron-source-registry.md) 刚刚删掉的 `definitionToFunc` 就是这样一张被并发
Echo handler 无锁读写、会触发 `concurrent map read and map write` 崩溃的内存 map。本 issue 引入的
`taskID→schedulerJobID` map 面临**完全相同**的并发形态（`AddTask/PatchTask/DeleteTask` 都是并发
handler），因此**它必须自带锁**（`sync.Mutex` / `sync.Map`），否则就是把 Issue 1 刚消除的数据
竞争原样重造一遍。

对比之下，现在这版「持久化到 DB 列」的实现**反而不需要锁**：每次操作都从 DB 行现读
`CronServiceJobID`，天然无共享可变内存状态。所以本 issue 的净效果是：**拿掉一个跨重启无意义、
但并发安全的 DB 列，换来一张有用但需要加锁的内存 map**。owner 已知情并决定照做（业务态与运行态
不该混存，语义更干净），但这条代价——「省下一个 DB 列 = 欠下一把锁」——必须在写 plan 时明确保留，
不能悄悄当成纯收益。

- 不改 `RemoveCrawlJob` / gocron 封装本身（`pkg/cron/cron.go`），只改 job id 的来源。
- 不引入跨进程共享（内存 map 单进程即可；本服务是单实例部署）。
- 不顺带动 `Type` 列（那是 [Issue 3](2026-07-12-cron-kind-string.md)）。
