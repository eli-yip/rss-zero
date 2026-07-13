---
title: "删除持久化的 cron_service_job_id 列，改由内存维护 taskID→schedulerJobID"
issue: docs/issues/2026-07-12-cron-drop-jobid-column.md
status: done
areas: [cron, db, controller, migrate]
updated: "2026-07-13"
---

# PLAN: 删除持久化的 cron_service_job_id 列，改由内存维护 taskID→schedulerJobID

> 实现顺序：本 issue 在 [Issue 3（kind 字符串）](2026-07-12-cron-kind-string.md) 落地之后进行。
> 下述行号以 Issue 3 合并后的代码为基线，来源字段名用 `Kind`。

## 目标

解决 [issue](../issues/2026-07-12-cron-drop-jobid-column.md)：不再把 gocron 的进程内 job UUID
持久化到 `cron_tasks.cron_service_job_id`，删掉该列；改由一张带锁的内存
`map[taskID]schedulerJobID` 维护「definition → 当前调度器 job」，`AddCrawlJob` 后写入、
`RemoveCrawlJob` 前读取。不改 gocron 封装本身、不引入跨进程共享。

## 当前症结

`cron_service_job_id`（`definition.go:21`）持久化的是**调度器进程内的 UUID**：每次进程启动，
`addJobToCronService`（`cmd/server/cron.go:145`）对所有 definition 重新 `AddToScheduler`，
`AddCrawlJob` 生成**新** UUID，`AddToScheduler`（`registry.go:91`）再 `PatchDefinition` 把它写回
DB 覆盖旧值。**上次进程写进 DB 的 UUID，在下次进程里既不存在于新调度器、也会被启动流程立刻覆盖**
——一个「运行态」被当成「业务态」持久化的纯技术债。

该列在进程内唯一消费点是摘 job 时传给 `RemoveCrawlJob`：`PatchTask`（`task.go:135`）与
`DeleteTask`（`task.go:174`），两处读的都是刚 `GetDefinition` 拿到的行里的 `CronServiceJobID`。

## 关键决策

### 1. 新增带锁的 `JobIndex`（`internal/controller/job/jobindex.go`）

**选择**：一个最小的加锁 map 封装，单进程内共享一个实例。

```go
type JobIndex struct {
    mu sync.Mutex
    m  map[string]string // taskID(definition id) -> scheduler job id
}

func NewJobIndex() *JobIndex { return &JobIndex{m: make(map[string]string)} }
func (j *JobIndex) Set(taskID, jobID string) { j.mu.Lock(); defer j.mu.Unlock(); j.m[taskID] = jobID }
func (j *JobIndex) Get(taskID string) (string, bool) { j.mu.Lock(); defer j.mu.Unlock(); v, ok := j.m[taskID]; return v, ok }
func (j *JobIndex) Delete(taskID string) { j.mu.Lock(); defer j.mu.Unlock(); delete(j.m, taskID) }
```

**理由（= issue 里强制保留的代价）**：`AddTask`/`PatchTask`/`DeleteTask` 是并发 Echo handler，
本 map 面临与 Issue 1 刚删掉的 `definitionToFunc` **完全相同**的并发形态，因此**必须自带锁**，否则
就是把刚消除的 `concurrent map read and map write` 原样重造。相较之下，现在这版「持久化到 DB 列」
天然无共享可变内存、不需要锁。**本 issue 的净效果是：拿掉一个跨重启无意义但并发安全的 DB 列，
换来一张有用但需加锁的内存 map——省下一个 DB 列 = 欠下一把锁**。owner 已知情并决定照做（业务态
与运行态不该混存，语义更干净）。用 `sync.Mutex` 足够（map 极小，不上 `sync.Map`）。

**锁粒度边界（评审提出，必须写清）**：逐方法加锁**只消除 map 的数据竞争**（`concurrent map
read/write` fatal）——这正是本 issue 验收明确要求的那一项。它**不**串行化「Patch = Get 旧 →
Set 新 → Remove 旧」「Delete = Get → Remove → DB delete → Delete」这种跨多步的**逻辑**生命周期：
同一 taskID 的两个并发 Patch 仍可能交错，导致 scheduler job 泄漏或删掉刚写入的新映射。**但这层
逻辑竞态是既有行为、并非本 issue 引入**：现版「每次从 DB 行现读 `CronServiceJobID`」的实现有
**完全相同**的交错窗口（两个并发 Patch 同样各读旧值、各加新 job、各摘旧 job）。本 issue 是
「等价搬家」（issue 原话：并发面并没有减少），因此**不在本 issue 扩大锁范围去关这个既有窗口**
（YAGNI；真要关是另开 issue 做 per-taskID 串行化）。plan 只承诺「消除数据竞争」，不承诺「串行化
生命周期」，以免验收对不上话。

### 2. `JobIndex` 实例的生命周期与穿线

**选择**：在 `setupCronCrawlJob`（`cmd/server/cron.go:28`）里
`jobIndex := jobController.NewJobIndex()`，

- 传给 `addJobToCronService`（startup），它调 `AddToScheduler(cronService, jobIndex, …)`；
- 由 `setupCronCrawlJob` 一并返回，穿过 `main.go` → `setupEcho`（`echo.go`）→ `NewController`，
  让 `PatchTask`/`DeleteTask` 能读/删同一实例。

**唯一写入点约定**：`jobIndex.Set` **只在 `AddToScheduler` 内部**发生（决策 3），caller
（`addJobToCronService` / `addTaskToCronService`）只负责把 `jobIndex` 传进去、不自己 `Set`。
避免「谁写 map」两处描述打架。

**理由**：startup 期（`addJobToCronService`）只写、请求期（Controller）读写删，二者必须共享同一张
map，故实例在两者共同的上游 `setupCronCrawlJob` 创建并向下传，与现有 `cronService` 穿线路径一致。
注意 `addJobToCronService` 仍保留 `cronDBService` 形参（它还调 `cronDBService.GetDefinitions()`，
`cron.go:146`），本 issue 只是给它**新增** `jobIndex` 形参，不删 `cronDBService`。

### 3. `AddToScheduler` 写 map 取代写 DB

**选择**：`registry.go:86` 的 `AddToScheduler` 去掉 `cronDBService` 形参、去掉
`PatchDefinition(def.ID, nil,nil,nil, &jobID)`，改收 `jobIndex *JobIndex` 并
`jobIndex.Set(def.ID, jobID)`：

```go
func AddToScheduler(cronService *cron.CronService, jobIndex *JobIndex, spec SourceSpec, deps BuildDeps, def *cronDB.CronTask) (jobID string, err error) {
    fn := spec.Build(deps, def, nil)
    if jobID, err = cronService.AddCrawlJob(spec.JobName(), def.CronExpr, fn); err != nil {
        return "", fmt.Errorf("failed to add crawl job: %w", err)
    }
    jobIndex.Set(def.ID, jobID)
    return jobID, nil
}
```

两个调用点同步改参：`addJobToCronService`（startup，`cron.go:156`）与 `addTaskToCronService`
（`task.go:72`，Controller 持有 `h.jobIndex`）。

### 4. `PatchTask` / `DeleteTask` 改从 map 取 jobID

- **PatchTask**（`task.go`）：现流程是先 `addTaskToCronService`（拿 newJobID）再
  `RemoveCrawlJob(originalTaskDef.CronServiceJobID)`（`task.go:135`）。改为：**先**
  `oldJobID, ok := h.jobIndex.Get(req.ID)` 存下旧值，**再** `addTaskToCronService`（内部 `Set`
  覆盖成新 job），**再** 仅当 `ok` 时 `RemoveCrawlJob(oldJobID)`。取旧值必须在 `Set` 覆盖之前。
  **保留** 开头的存在性校验：现有 `originalTaskDef, err := GetDefinition(req.ID)`（`task.go:108`）
  兼作「任务是否存在」的前置校验（不存在则 400），删列后它不再供 jobID，故**改写成
  `_, err := GetDefinition(req.ID)`**（评审补：否则 `originalTaskDef` 变未使用、编译不过）——
  保留这次查询本身，删掉的只是对 `.CronServiceJobID` 的读取，不能连查询一起删（会把「patch 不
  存在的 taskID」从 400 退化成静默走完，是行为回归）。
- **DeleteTask**（`task.go:167` 起）：`jobID, ok := h.jobIndex.Get(taskID)` → 仅当 `ok`
  时 `RemoveCrawlJob(jobID)`（现 `task.go:174`）→ 删 definition 成功后 `h.jobIndex.Delete(taskID)`。
- **`Get` 缺项处理（评审补）**：`jobIndex.Get` 返回 `ok=false` 时**不得**把空串喂给
  `RemoveCrawlJob`——`cron.go` 侧会拿空串去解析 UUID 必然报错。缺项即「调度器里本就没有这个 job」，
  按 no-op 跳过 `RemoveCrawlJob` 并记一条 warning 即可，不当错误中断流程。
- **中途失败（评审提出，记录不扩治）**：Patch/Delete 分「改 DB / 摘旧 job / 加新 job」多步，中途
  失败可能留下「DB 已改但 job 没换」「新旧 job 并存」等不一致——这与现版实现同构、非本 issue 引入，
  本 issue 不加补偿事务（同「等价搬家」原则）；此处仅点名，若要治另开 issue。

### 5. DB 层删字段

- `CronTask` 删 `CronServiceJobID` 字段（`definition.go:21`）。
- `PatchDefinition` 删第五参 `cronServiceJobID *string` 及函数体写入（`definition.go:52,68-70`）；
  `CronTaskIface.PatchDefinition`（`definition.go:28`）签名同步。删第五参后**全部调用点**（评审补，
  逐一核对）：
  - `AddToScheduler` 里的 `PatchDefinition(def.ID, nil,nil,nil, &jobID)`（`registry.go:91`）——**整条
    删除**（由决策 3 改成写 map，不是改签名）；
  - `PatchTask` 里的 `PatchDefinition(req.ID, req.CronExpr, req.Include, req.Exclude, nil)`
    （`task.go:115`）——去掉末尾 `nil`；
  - 测试 fake 的实现与调用（`handler_test.go:50` 签名、`:67` 写 `CronServiceJobID`、`:218` 调用、
    `:256/264/266` 断言 `CronServiceJobID`）——需机械更新。测试由 owner 负责，plan 在此点名其存在，
    避免改完源码后测试包编译不过被当意外。

### 6. Migration: DROP COLUMN

新增 `internal/migrate/20260714000001.go`（版本号取大于 Issue 3 migration 的递增值，
`YYYYMMDDHHMMSS`）：照现有样板（`20260711000000.go`）在 `init()` 里
`Register(Migration{Version, Name:"cron-drop-jobid-column", Auto:true, RequiresPredecessors:false, Run:...})`，
`Run` 执行单条 `ALTER TABLE cron_tasks DROP COLUMN IF EXISTS cron_service_job_id`。删列与 Issue 3 的
kind migration **相互独立、无先后依赖**，故 `RequiresPredecessors:false`（版本号只保证枚举顺序，不构成
依赖）。AutoMigrate 不删列（struct 去掉字段后旧列仍在），故需本 migration 显式删；`IF EXISTS` 保证
幂等（列已删则空跑）。

### 7. 明确不做

- 不改 `RemoveCrawlJob` / gocron 封装（`pkg/cron/cron.go`），只改 jobID 的来源。
- 不引入跨进程共享（单实例部署，内存 map 足够）。
- 不动 `Kind` 列（那是 Issue 3，已先落地）。
- **AddTask 失败回滚**（`task.go:51` `DeleteDefinition`）无需 `jobIndex.Delete`：`Set` 只在
  `AddCrawlJob` 成功后执行（决策 3），回滚发生时 map 尚未写入，无残留。

## 代码落点

- `internal/controller/job/jobindex.go`（新增）：`JobIndex` 类型 + `New/Set/Get/Delete`。
- `internal/controller/job/registry.go`：`AddToScheduler` 换 `cronDBService`→`jobIndex`，写 map。
- `internal/controller/job/controller.go`：`Controller` 加 `jobIndex *JobIndex` 字段，
  `NewController` 加形参、`buildDeps` 无关不动。
- `internal/controller/job/task.go`：`addTaskToCronService` 传 `h.jobIndex`；`PatchTask`/`DeleteTask`
  改从 map 取 jobID（PatchTask 保留 `originalTaskDef` 存在性校验）；`PatchDefinition` 调用去掉末尾 `nil`。
- `pkg/cron/db/definition.go`：删 `CronServiceJobID` 字段、`PatchDefinition` 第五参与接口签名。
- `internal/controller/job/handler_test.go`（**已存在**，评审补）：fake 的 `PatchDefinition` 签名
  （`:50`）、`CronServiceJobID` 写入/断言（`:67,256,264,266`）、`:218` 调用需机械更新；测试由
  owner 负责，plan 点名其存在以免编译意外。
- `cmd/server/cron.go`：`setupCronCrawlJob` 建 `jobIndex` 并返回；`addJobToCronService` 新增
  `jobIndex` 形参并 `Set`；调整返回值链路。
- `cmd/server/main.go` / `cmd/server/echo.go`：`jobIndex` 穿线到 `NewController`。
- `internal/migrate/20260714000001.go`（新增）：`DROP COLUMN cron_service_job_id`。

## 实施步骤（对应提交）

1. **JobIndex + 穿线**：新增类型；`setupCronCrawlJob` 建实例并穿线到 `addJobToCronService` 与
   `NewController`。
2. **切来源**：`AddToScheduler` 写 map 不写 DB；`PatchTask`/`DeleteTask` 从 map 取 jobID。
3. **删列**：`CronTask` 去字段、`PatchDefinition` 去第五参；新增 DROP 列 migration。
4. **文档 + 评审**：更新 ARCHITECTURE / PROGRESS，`go build ./...`、`just lint`，跑 `/code-review`。

## 测试（owner 负责）

> owner 已明确：测试由 owner 编写；本节仅列**需覆盖点清单**。

- `JobIndex` 方法级并发（`go test -race`）：多 goroutine 对同一 taskID 循环 `Set`/`Get`/`Delete`，
  断言无 race、无 `concurrent map` fatal。
- **issue 验收要求的 handler 级并发**（`go test -race`，评审补）：issue 明确要求「覆盖 add/patch/delete
  同 taskID」。需对同一 taskID 并发打 `AddTask`/`PatchTask`/`DeleteTask`（用内存 fake `cronDB.DB` +
  `cron.NewCronService` + no-op 的 `registry`，避免真爬），断言无数据竞争。注意：本测试验证的是「无
  数据竞争」，**不**断言无逻辑交错（那层是既有窗口、决策 1 已说明不在本 issue 关）。
- add/patch/delete 端到端（内存 fake `cronDB.DB` + `cron.NewCronService`）：
  - Add 后 `jobIndex` 有该 taskID→jobID；
  - Patch 后旧 job 被摘、`jobIndex` 指向新 jobID；patch 不存在的 taskID 仍返回 400；
  - Delete 后 job 被摘且 `jobIndex` 无该 taskID。
- migration：起一张含 `cron_service_job_id` 列的 `cron_tasks`，跑 migration 后断言列消失；
  空跑（列已不存在）幂等。

- 验证命令：`go test -race ./internal/controller/job/...`、
  `go test ./pkg/cron/... ./internal/migrate/...`、`go build ./...`、`just lint`。

## 待更新文档

- [ ] `docs/ARCHITECTURE.md`：cron 段说明 jobID 由内存 `JobIndex` 维护、不再持久化。
- [ ] `docs/PROGRESS.md`：完成 / 合并结论（与 doc 变更同一提交）。
- [ ] 本 issue：合并时 `status: closed`。
