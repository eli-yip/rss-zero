# PLAN: 迁移注册表与启动自动迁移

实现 [SPEC 2026-06-21-01](../issues/2026-06-21-migration-registry.md)。本分支
`feat-migration-registry`（off master）只交付框架；注册表留空，用测试里的假迁移验证。小步提交。

依赖事实（已核实）：

- AutoMigrate 在 [internal/migrate/db.go `MigrateDB`](../../internal/migrate/db.go)，
  启动时由 [cmd/server/main.go:134](../../cmd/server/main.go) 调用（在 DB init 之后、`setupEcho` 之前）。
- 迁移控制器 [controller/migrate/migrate.go](../../internal/controller/migrate/migrate.go) 构造仅需
  `(logger, db)`；路由在 [cmd/server/echo.go `registerMigrate`](../../cmd/server/echo.go)。
- 集合库：`github.com/deckarep/golang-set/v2`（`mapset`，refmt 已用）已在 go.mod。

核心逻辑做成**纯函数**（不碰 DB），便于单测；DB 读写是薄封装。

## 步骤

### 步骤 1：完成记录表 + 集合读写

- 新建 [internal/migrate/record.go](../../internal/migrate/record.go)：
  - `type SchemaMigration{ Version int64 gorm:"column:version;primaryKey"; Name string; AppliedAt time.Time }`，
    `TableName()=="schema_migrations"`。
  - `loadApplied(db) (mapset.Set[int64], error)`：`Pluck("version")` → 装入 `mapset.NewSet[int64]()`。
  - `recordApplied(db, version int64, name string) error`：`Create(&SchemaMigration{...now...})`。
- 在 [db.go `MigrateDB`](../../internal/migrate/db.go) 的 AutoMigrate 列表加 `&SchemaMigration{}`。

`just lint` + `go build ./...`。提交 `feat(migrate): schema_migrations record table`。

### 步骤 2：注册表 + 校验（纯函数 + 全局）

新建 [internal/migrate/registry.go](../../internal/migrate/registry.go)：

```go
type Migration struct {
    Version              int64
    Name                 string
    Auto                 bool
    RequiresPredecessors bool
    Run                  func(db *gorm.DB, logger *zap.Logger) error
}
var registry []Migration
func Register(m Migration) { registry = append(registry, m) }
```

- `validateRegistry(ms []Migration) error`：版本号 `<=0` 或重复 → error（含冲突版本）。
- 文件顶部注释写 **append-only 纪律 + 时间戳 `YYYYMMDDHHMMSS`** 约定。
- 纯判定函数（均不碰 DB，入参显式传 `[]Migration` 与 `mapset.Set[int64]`）：
  - `predecessorsDone(m, all, applied) bool`：所有 `Version<m.Version` 的注册项都在 applied。
  - `eligible(m, all, applied) bool`：`!applied.Contains(m.Version) && (!m.RequiresPredecessors || predecessorsDone)`。
  - `isOutOfOrder(m, applied) bool`：`!applied.Contains(m.Version) && applied 非空 && m.Version < max(applied)`。
  - `pending(all, applied, autoOnly) []Migration`：过滤 eligible（autoOnly 再要求 `Auto`）后按 `Version` 升序。

单测 [registry_test.go](../../internal/migrate/registry_test.go)：`validateRegistry`（正常/重复/非正）、
`eligible`/`predecessorsDone`（前置未完成被挡）、`isOutOfOrder`、`pending` 排序与过滤。**纯函数无需 DB**。

`just lint` + `go test ./internal/migrate/`。提交 `feat(migrate): migration registry and eligibility rules`。

### 步骤 3：运行器（RunAuto / 手动 / 状态）

新建 [internal/migrate/runner.go](../../internal/migrate/runner.go)：

- `RunAuto(db, logger)`：`mustValidate()`（校验失败 panic）→ `loadApplied` → `all := registry` →
  按 `Version` 升序遍历：对 `isOutOfOrder` 记 warning；对 `eligible && Auto` 调 `runOne`。失败只记日志。
- `runOne(db, logger, m)`：`recover` 包裹 `m.Run`；nil → `recordApplied`；err/panic → 记 error、不记录。
- `RunVersion(db, logger, version) error`：找到该版本，校验 `eligible`（不满足前置/已完成则返回说明性 error），`runOne`。
- `RunPending(db, logger)`：对所有 `eligible`（含非 auto）按序 `runOne`。
- `Status(db) ([]MigrationStatus, error)`：合并 registry 与 applied，给出
  `{Version,Name,Auto,RequiresPredecessors,Completed,AppliedAt}`（升序）。
- 并发：单实例，不加锁；注释留多实例需 `pg_advisory_xact_lock` 的扩展点。

单测：用假 `Migration`（`Run` 改内存计数器/可控返回 err）跑 `RunAuto`/`RunPending` 的**纯调度**部分——
把「选哪些、什么顺序」抽成 `pending()` 已在步骤 2 测；runner 这里补一个用 in-memory 记录器（传入
`applied` 与一个 `record func`）验证「成功才记录、err 不记录、前置门控、乱序 warning 调用」。
DB 薄封装（loadApplied/recordApplied/Status）不单测（依赖 Postgres），靠步骤 5 端点 + 部署观测。

> 设计：把 runner 的调度核心写成 `runPlan(plan []Migration, record func(Migration) error, logger)`，
> 纯逻辑、可测；`RunAuto/RunVersion/RunPending` 负责 loadApplied + 组装 plan + 调 `runPlan`。

`just lint` + `go test ./internal/migrate/`。提交 `feat(migrate): startup/manual migration runner`。

### 步骤 4：启动接线

[cmd/server/main.go](../../cmd/server/main.go)：在 `migrate.MigrateDB` 成功之后加
`migrate.RunAuto(dbService, logger)`（其内部不返回阻断性错误；失败只记日志）。

`just lint` + `go build ./...`。提交 `feat(migrate): run auto migrations on startup`。

### 步骤 5：控制器 + 路由

- [controller/migrate/migrate.go](../../internal/controller/migrate/migrate.go) 加：
  - `RegistryStatus(c)` → `migrate.Status(h.db)` → JSON（`httputil.NewResp`）。
  - `RunVersion(c)` → 解析 `:version`（`strconv.ParseInt`），`go migrate.RunVersion(...)` 或同步返回结果；
    沿用现有 `httputil.NewMessage`。
  - `RunPending(c)` → `go migrate.RunPending(...)`。
- [echo.go `registerMigrate`](../../cmd/server/echo.go) 加三条路由（`GET /registry`、`POST /run/:version`、
  `POST /run-pending`），命名同风格。旧路由不动。

`just lint` + `go build ./...`。提交 `feat(migrate): registry/run endpoints`。

### 步骤 6：收尾

- `just lint` 全绿；`go test ./internal/migrate/` 通过；`go build ./...`。
- 更新 [docs/PROGRESS.md](../PROGRESS.md) 状态为「实现完成，待 review」。
- 请作者 review；批准后 squash 合并 master（或按需），再**合并进 `feat-paid-content-flag`**，
  在该分支把 `Migrate20260620` 注册为 `Version=20260620000000/Auto`、删其专属端点（SPEC §6，不在本分支做）。

## 风险 / 注意

- **纯函数边界**：调度/资格/排序/乱序判定全做成纯函数本地单测；DB 仅 loadApplied/recordApplied/Status
  三个薄封装，避免在单测里依赖 Postgres。
- **注册表为空**：本分支 `registry` 无条目，`RunAuto` 启动即 no-op；正确。测试用假迁移切片覆盖逻辑。
- **panic 边界**：仅 `validateRegistry` 失败（开发期错误）panic；迁移自身 `Run` 的 panic 被 `runOne`
  recover，不影响启动。
- **append-only**：注释强约束，避免删除/改版本号破坏历史与乱序判定。
