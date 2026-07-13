---
title: "cron 任务类型从 int(iota) 改为字符串 Kind"
issue: docs/issues/2026-07-12-cron-kind-string.md
status: done
areas: [cron, db, controller, migrate]
updated: "2026-07-13"
---

# PLAN: cron 任务类型从 int(iota) 改为字符串 Kind

## 目标

解决 [issue](../issues/2026-07-12-cron-kind-string.md)：把 `cron_tasks.type` 从位置相关的
`int(iota)` 改成自描述的字符串 `kind`，让 DB 行不再受 const 顺序影响。借
[Issue 1](2026-07-12-cron-source-registry.md) 的注册表已经落地这一点，把「API 字符串、注册表键、
DB 列」三者统一成同一个字符串——`TypeStrToInt` / `TypeIntToStr` 两个转换层随之整条删除。不动
API 契约、不动 `cron_service_job_id`（那是 [Issue 2](2026-07-12-cron-drop-jobid-column.md)）、
不动各源 `BuildCrawlFunc` 与 gocron 封装。

## 当前症结

`CronTask.Type` 是 `int`（`pkg/cron/db/definition.go:17`），值来自 `definition.go:34` 起的
`iota` 块（`TypeZsxq=0 / TypeZhihu=1 / TypeXiaobot=2 / TypeGitHub=3`）。重排、插入或删除这个块
里任何一项，都会**静默改写已存历史行的语义**：DB 里的 `2` 原本是 xiaobot，若有人在前面插一个新源，
`2` 就悄悄变成别的源，无编译错误也无运行报错。

Issue 1 已把「int ↔ 字符串」塌成注册表查询：`registry.go` 里 `SourceSpec` 同时持有 `Type int`
（DB 键）与 `Name string`（API 字符串），`SpecByType` / `specByName` / `TypeStrToInt` /
`TypeIntToStr` 四个 helper 在两种表示间来回翻译。但只要 DB 存的还是 int，这层翻译就得留着。一旦
DB 直接存 `Name` 字符串，`Type` 字段与四个 helper 里的三个（`SpecByType` / `TypeStrToInt` /
`TypeIntToStr`）就全是多余的间接。

## 关键决策

### 1. `SourceSpec` 合并 `Type`+`Name` 为单一 `Kind` 字段

**选择**：删掉 `SourceSpec.Type int`，把 `Name string` 改名为 `Kind string`。API 字符串、注册表
键、DB 列本就该是同一个值，两个字段是历史残留的一物两名。

```go
type SourceSpec struct {
    Kind      string // "zsxq"/"zhihu"/"xiaobot"/"github"：API 字符串 = 注册表键 = DB kind 列
    Resumable bool
    Build     func(deps BuildDeps, def *cronDB.CronTask, resume *ResumeInfo) CrawlFunc
}

func (s SourceSpec) JobName() string { return s.Kind + "_crawl" }

var registry = []SourceSpec{
    {Kind: "zsxq",    Resumable: true,  Build: buildZsxq},
    {Kind: "zhihu",   Resumable: true,  Build: buildZhihu},
    {Kind: "xiaobot", Resumable: false, Build: buildXiaobot},
    {Kind: "github",  Resumable: false, Build: buildGitHub},
}
```

**理由**：Issue 1 的 `Name` 值（`"zsxq"` 等）与本 issue 要写进 DB 的 `kind` 值逐字相同，无需
新增字段，改名即可。`JobName()` 生成的 `zsxq_crawl` 等调度器名不变（原来就是 `Name+"_crawl"`）。

### 2. 查表 helper 从三个塌成一个 `SpecByKind`

**选择**：
- `specByName`（`registry.go:109`）改名并导出为 `SpecByKind(kind string) (SourceSpec, bool)`
  ——`cmd/server/cron.go` 跨包调用，必须导出。
- 删除 `SpecByType`（`registry.go:98`）、`TypeStrToInt`（`registry.go:119`）、
  `TypeIntToStr`（`registry.go:128`）。

**理由**：DB kind == API 字符串后，「按 int 查」「string↔int 互转」三件事全部退化为「按 kind
字符串查」这一件。`AddTask` 校验来源、`DeleteTask`/`ListTask` 回显来源、startup 与请求路径装载
job，统一走 `SpecByKind`。

### 3. `CronTask.Type int` → `Kind string`，`AddDefinition` 签名跟改

**选择**：
```go
type CronTask struct {
    // ...
    Kind     string `gorm:"column:kind;type:string"` // zsxq/zhihu/xiaobot/github
    // ...
}

// CronTaskIface：
AddDefinition(kind string, cronExpr string, include, exclude []string) (id string, err error)
```
删除 `definition.go:34` 起的 `iota` const 块（`TypeZsxq…`）。`AddDefinition` 的 `taskType int`
形参改为 `kind string`，函数体 `Type: taskType` 改为 `Kind: kind`。

**理由**：DB 列是本 issue 的核心。`PatchDefinition` 不碰来源类型（它没有该参数），无需改动。

### 4. handler 去掉转换层（`internal/controller/job/task.go` + `job.go`）

- **AddTask**（`task.go:33`）：`TypeStrToInt(req.TaskType)` 换成纯校验
  `if _, ok := SpecByKind(req.TaskType); !ok { 400 }`；`AddDefinition(req.TaskType, …)`；
  内存装配的 `def` 用 `Kind: req.TaskType`（`task.go:47`）。
- **addTaskToCronService**（`task.go:68`）：`SpecByType(def.Type)` → `SpecByKind(def.Kind)`。
- **DeleteTask**（`task.go:186`）/ **ListTask**（`task.go:215,241`）：删掉 `TypeIntToStr(...)`
  调用，`TaskInfo.TaskType` 直接取 `def.Kind`（DB 值即 API 字符串）。这去掉了三处「转换失败
  返回 500」的错误分支。
- **StartJob**（`job.go:30`，评审补漏）：`spec, ok := SpecByType(definition.Type)` →
  `SpecByKind(definition.Kind)`。这是删 `SpecByType` 后会漏改、直接编译不过的调用点。

### 5. startup 路径（`cmd/server/cron.go`）

`resumeRunningJobs`（`cron.go:129`）与 `addJobToCronService`（`cron.go:152`）里的
`SpecByType(def.Type)` / `SpecByType(definition.Type)` → `SpecByKind(def.Kind)`；错误信息里的
`%d`（`cron.go:131,154`）改 `%s`。

`Name`→`Kind` 改名会波及四处日志/错误里的 `spec.Name`（`cron.go:137,139,158,160`，评审补漏），
全部改 `spec.Kind`（值不变，`"zsxq"` 仍是 `"zsxq"`）。

### 6. Migration：AutoMigrate 加列 → registry migration 回填 + 删旧列

新增 `internal/migrate/20260714000000.go`（版本号取大于现有最大 `20260713000000`（master 的
`zhihu-drop-content-text`）的递增值，避免与其撞版本，
`YYYYMMDDHHMMSS`）：

- 时序保证（已确认）：`MigrateDB` 的 `AutoMigrate`（`db.go:20`）先于 `RunAuto`（`main.go:142`）
  执行。struct 改成 `Kind string` 后，AutoMigrate 会自动**新增 `kind` 列**（可空、空串），
  但 gorm **不删** `type` 列、也不回填——这两件事正是本 migration 的职责。
- 注册方式与现有 migration 一致（`init()` 里 `Register(Migration{Version, Name, Auto:true,
  RequiresPredecessors:false, Run:...})`，样板见 `internal/migrate/20260711000000.go`）。本 migration
  与其它 migration 无先后依赖，`RequiresPredecessors:false`。
- migration 逻辑（幂等）：
  1. `if !db.Migrator().HasColumn(&cronDB.CronTask{}, "type") { return nil }`（已迁移则空跑）；
  2. **未知值保护**（评审补）：先探测 `type NOT IN (0,1,2,3)` 的行，若存在则 `return error`
     ——让 migration 失败重试而非把未知源静默写成 NULL 后连原始 `type` 一起删掉、丢失证据；
  3. 回填：`UPDATE cron_tasks SET kind = CASE type WHEN 0 THEN 'zsxq' WHEN 1 THEN 'zhihu'
     WHEN 2 THEN 'xiaobot' WHEN 3 THEN 'github' END WHERE type IS NOT NULL`
     （以 `type` 列有值为准，不依赖 AutoMigrate 给 `kind` 新列的初值是 NULL 还是空串）；
  4. `ALTER TABLE cron_tasks DROP COLUMN IF EXISTS type`。
- **映射常量必须钉死原 iota 顺序**（0→zsxq、1→zhihu、2→xiaobot、3→github）。删 iota 块前，把
  这份映射写进 migration；migration 是这份历史映射的唯一存放处，删块之后代码里不再有别处能对照。

### 6a. 迁移失败与回滚的行为约定（评审补）

- **回滚不是「无损可逆」**：迁移框架是 forward-only（`internal/migrate/registry.go:12` 明确标注）。
  若回滚到删列前的旧二进制，其 `AutoMigrate` 会重建一个空 `type` 列，旧代码按 `Type int` 读到零值
  会把所有任务解读成 `zsxq`（type=0）。因此本 issue 的正确「回退」方式是**前滚修复**（改回或补数据），
  而非换回旧二进制。plan 里不再声称「可回滚」，只说「独立 diff、可独立前滚」。
- **迁移失败 = 启动硬失败，且这是有意的**：`RunAuto` 失败只记日志、不 fatal
  （`runner.go` 内 non-fatal），但紧接着 `addJobToCronService`（`cron.go:145`）会用空 `kind` 走
  `SpecByKind` 查表失败并返回 error，使 `setupCronCrawlJob` 失败、进程起不来。这是**期望行为**
  （回填没完成就不该带着空 kind 上线，fail loud 优于静默跑错源）；单实例部署下可接受，无需额外
  完成度门闩。plan 显式记录此约定，避免评审把「起不来」当回归。

### 7. 明确不做

- 不改 API 契约：请求/响应的 `task_type` 本就是字符串，保持不变。
- 不动 `cron_service_job_id` 列与 `AddToScheduler` 里的 jobID 写回（那是 Issue 2）。
- 不动各源 `buildXxx` 闭包与 gocron 封装。
- 不与 Issue 1/2 合并：独立 diff、可独立前滚（**非**「无损可回滚」，见决策 6a）。

## 迁移风险

低。`cron_tasks` 是用户手动增删的动态任务定义，行数个位数级；回填是无歧义的确定映射。唯一要点是
映射常量与 iota 原顺序对齐（决策 6）。

## 代码落点

- `pkg/cron/db/definition.go`：`CronTask.Type`→`Kind`；`AddDefinition` 形参 int→string 及函数体；
  `CronTaskIface.AddDefinition` 签名；删 `iota` const 块。
- `internal/controller/job/registry.go`：`SourceSpec` 合并 `Type`+`Name`→`Kind`；registry 表；
  `JobName()` 用 `Kind`；`specByName`→导出 `SpecByKind`；删 `SpecByType`/`TypeStrToInt`/`TypeIntToStr`。
- `internal/controller/job/task.go`：`AddTask` 改纯校验 + `Kind` 装配；`addTaskToCronService`、
  `DeleteTask`、`ListTask` 改走 `SpecByKind` / `def.Kind`，删 `TypeIntToStr` 分支。
- `internal/controller/job/job.go`：`StartJob`（`job.go:30`）`SpecByType(definition.Type)`→`SpecByKind(definition.Kind)`。
- `cmd/server/cron.go`：`resumeRunningJobs` / `addJobToCronService` 的 `SpecByType`→`SpecByKind`；
  `%d`（`cron.go:131,154`）改 `%s`；`spec.Name`（`cron.go:137,139,158,160`）→`spec.Kind`。
- `internal/controller/job/registry_test.go`、`handler_test.go`（**已存在**，评审补）：引用了将被删的
  `SpecByType`/`TypeStrToInt`/`TypeIntToStr`/`SourceSpec.Type`（`registry_test.go:27-63`）等，删符号后
  这些测试**编译不过**，需机械更新为 `SpecByKind`/`.Kind`。测试内容由 owner 负责，但 plan 在此点名
  它们的存在，避免「改完源码但测试包炸了」被当意外。
- `internal/migrate/20260714000000.go`（新增）：未知值保护 + 回填 int→kind + `DROP COLUMN type`。

## 实施步骤（对应提交）

1. **DB 层**：`definition.go` 改 `Kind`、`AddDefinition`、删 iota。
2. **注册表**：`registry.go` 合并字段、`SpecByKind`、删三个 helper。
3. **调用方**：`task.go` + `cmd/server/cron.go` 全部切到 `SpecByKind` / `def.Kind`，去转换分支。
4. **migration**：新增回填 + 删列 migration。
5. **文档 + 评审**：更新 ARCHITECTURE / PROGRESS，`go build ./...`、`just lint`，跑 `/code-review`。

## 测试（owner 负责）

> owner 已明确：测试由 owner 编写；本节仅列**需覆盖点清单**供 owner 参照，plan 不铺测试基建。

- 注册表：`SpecByKind` 四源命中、未知 kind 返回 `!ok`；`JobName()` 得 `<kind>_crawl`；
  `Resumable` 仅 zsxq/zhihu 为 true。
- migration：需复刻真实顺序「legacy 表 (有 `type` 无 `kind`) → AutoMigrate 加 `kind` 列 →
  registry migration」，否则只建含 `type` 的表直接 UPDATE 会因缺 `kind` 列失真；跑后断言 `kind`
  回填正确、`type` 列消失；空跑（无 `type` 列）幂等；含未知 `type`(如 9) 时 migration 报错、不删列。
- handler：`AddTask` 未知 `task_type` 返回 400；`ListTask`/`DeleteTask` 回显的 `task_type`
  与写入一致。

- 验证命令：`go test ./internal/controller/job/... ./pkg/cron/... ./internal/migrate/...`、
  `go build ./...`、`just lint`。

## 待更新文档

- [ ] `docs/ARCHITECTURE.md`：cron 段把来源键从 int 改述为 kind 字符串。
- [ ] `docs/PROGRESS.md`：完成 / 合并结论（与 doc 变更同一提交）。
- [ ] 本 issue：合并时 `status: closed`。

## 后续项

- [Issue 2](2026-07-12-cron-drop-jobid-column.md)：删持久化的 `cron_service_job_id` 列，改内存
  map。本 issue 落地后接着做。
