# SPEC: 迁移注册表与启动自动迁移

- 日期：2026-06-21
- 状态：已 review（决策已定），待 PLAN

## 背景 / 动机

现状（[internal/migrate](../../internal/migrate)）每个一次性迁移都是一个裸函数，
通过 `POST /api/v1/migrate/<date>` 手动触发（[cmd/server/echo.go registerMigrate](../../cmd/server/echo.go)、
[controller/migrate](../../internal/controller/migrate)）。问题：

- **必须人工触发**：部署后要记得去敲端点，容易漏；多环境/多次部署易错。
- **没有「已执行」记录**：靠迁移自身幂等兜底，无法回答「这个库跑到哪了」。
- **没有顺序/依赖保证**：函数之间没有声明先后，全靠人脑。

业内成熟做法（**Flyway / Rails ActiveRecord / goose / gormigrate / Alembic**）都是同一套：
版本表记录已应用版本 + 启动时按序应用未执行的迁移。本 SPEC 取这套思路，并按需求加两个
旋钮（`Auto` 自动开关、`RequiresPredecessors` 前置门控），覆盖「简单的应自动、危险的留手动」。

### 关键决策：时间戳版本号 + 已完成「集合」

完整性**不靠序号连续**，而靠「已完成集合 + 每次启动把未完成的全部按版本升序执行」：

> 每次启动计算 `注册表 − 已完成集合`，按 `Version` 升序逐个执行其中合格者。

任何「更早但没跑过」的迁移必然仍在该差集里，下次启动照样被执行——**不会漏，也不要求版本连续**。
因此采用：

- **`Version int64`，形如 `YYYYMMDDHHMMSS`**（沿用现有 `20240905` 的日期风格，精确到秒避免同日撞、
  且时间戳天然唯一，免去跨分支撞号与合并调号）。
- **已完成记录用集合**（`schema_migrations` 每应用一条插一行），用 go.mod 已有的
  `github.com/deckarep/golang-set/v2`（`mapset`）在内存里表达。
- 放弃「严格 +1 / 连续」校验；只保留**版本号去重校验**（重复 → panic）。
- 时间戳的代价是**相对顺序**的边角：若较晚版本 B 已执行，之后才补一个更早版本 A，A 仍会被执行
  但在 B 之后。两道防线：`RequiresPredecessors`（同一次启动里更早版本会先升序执行）+ **乱序 warning**
  （某未完成迁移版本 < 已完成集合最大版本时记 warning，照跑、不静默）。

### 与业内方案的对照

| 维度       | 成熟方案                     | 本设计                                                 |
| ---------- | ---------------------------- | ------------------------------------------------------ |
| 版本标识   | 时间戳（goose/Flyway）或序号 | **`int64` 时间戳 `YYYYMMDDHHMMSS`**                    |
| 版本记录   | `schema_migrations` 表       | 同名表 + 内存 `mapset` 集合                            |
| 触发       | CLI / 启动时全部应用         | 启动自动应用「auto 且合格」者；非 auto 留手动端点      |
| 乱序       | Flyway outOfOrder 检测       | **warning（照跑）** + `RequiresPredecessors` 门控      |
| 方向       | up/down                      | **仅 forward**（无 down）                              |
| schema DDL | 同框架管理                   | **仍由 GORM AutoMigrate 管**；注册表只管一次性数据迁移 |

## 目标 / 非目标

目标：

- 注册表声明迁移：`{Version, Name, Auto, RequiresPredecessors, Run}`。
- 注册校验：版本号唯一，重复则启动 **panic**（不再要求连续）。
- `schema_migrations` 表记已完成版本集合；启动自动执行「未完成 且 Auto 且 前置满足」者。
- 保留手动触发（`Auto=false` 或补跑）与只读状态查询；乱序记 warning。
- 仅把 **20260620（知乎付费提示回填）** 纳入注册表（合并进 `feat-paid-content-flag` 后做）；
  其余旧迁移（20240905/20240929/20250530/20260612）保留旧手动端点、**不进注册表**。

非目标：down/回滚；接管 schema DDL；改动旧迁移行为与端点；多实例分布式锁（见 §5）。

## 设计

### 1. 迁移描述与注册表

```go
// internal/migrate/registry.go
type Migration struct {
    Version              int64  // YYYYMMDDHHMMSS，唯一
    Name                 string // 人读名，如 "zhihu-paid-notice-backfill"
    Auto                 bool   // 启动时是否自动执行
    RequiresPredecessors bool   // 仅当所有更小 Version 都已完成才有资格执行
    Run                  func(db *gorm.DB, logger *zap.Logger) error
}

func Register(m Migration) { registry = append(registry, m) } // 各迁移文件 init() 调用
```

- **校验** `validateRegistry()`：版本号去重，发现重复或非正 → `panic`（开发期暴露）。
- **append-only 纪律**：已发布/已应用的迁移不得删除或改版本号。写进注册表文件顶部注释。

### 2. 完成记录表（社区惯例命名）

```go
type SchemaMigration struct {
    Version   int64     `gorm:"column:version;primaryKey"`
    Name      string    `gorm:"column:name;type:text"`
    AppliedAt time.Time `gorm:"column:applied_at;type:timestamptz"`
}
func (SchemaMigration) TableName() string { return "schema_migrations" }
```

启动时 `SELECT version FROM schema_migrations` 载入到 `mapset.Set[int64]`。某版本在集合中即已完成。

### 3. 执行资格与运行

「**合格执行**」当且仅当：

1. 自身未完成（`Version` 不在已完成集合）；且
2. `RequiresPredecessors == false`，**或** 注册表中所有 `Version' < Version` 者均已完成。

启动自动迁移额外要求 `Auto == true`。手动触发某 Version 同样要满足 1、2（不绕过前置）。

`runOne(m)`：

```
若已完成 → 跳过
若 RequiresPredecessors 且前置未全完成 → 跳过 + 记日志（waiting for predecessors）
recover 包裹 m.Run(db, logger)
  nil   → INSERT schema_migrations{version,name,now}
  err/panic → 不记录（下次可重试），记 error 日志
```

- **幂等要求**：每个 `Run` 必须幂等（与现有迁移一致）；记录在成功后写入，不强制与迁移同事务
  （数据回填多为分批，单一大事务不现实——与 goose 非事务迁移同策略）。需要原子的小迁移可自行
  在 `Run` 内开事务并把记录写进同一事务。

### 4. 启动接线、乱序检测与端点

- 启动：`cmd/server` 在 DB 初始化 + `AutoMigrate`（含建 `schema_migrations` 表）之后、对外服务前调
  `migrate.RunAuto(db, logger)`：`validateRegistry()` → 载入已完成集合 → 按 `Version` **升序**遍历：
  - **乱序检测**：遇到未完成且 `Version < max(已完成集合)` 者，记 warning（仍按资格执行）。
  - 执行 `Auto` 且合格者。
  - **失败只记日志，不阻断启动**（决策）。
- 端点（沿用 `/api/v1/migrate` 分组、鉴权不变）：
  - `GET  /api/v1/migrate/registry` — 列出 `{version,name,auto,requiresPredecessors,completed,applied_at}`。
  - `POST /api/v1/migrate/run/:version` — 手动触发某版本（满足资格才跑）。
  - `POST /api/v1/migrate/run-pending` — 触发所有合格者（含非 auto），用于补跑。
  - 旧 `POST /api/v1/migrate/20240905` 等保留不动。

### 5. 并发 / 单实例

生产为**单实例**（compose 一个 `rss-zero`），且 `RunAuto` 启动时只同步调用一次、靠已完成集合判重，
**不加分布式锁**（决策）。注释注明：将来上多实例需加 Postgres advisory lock（`pg_advisory_xact_lock`）。

### 6. 首个注册项（合并到 feat-paid-content-flag 后）

把现有 `Migrate20260620`（付费提示回填）改造为注册项：

- `Version=20260620000000, Name="zhihu-paid-notice-backfill", Auto=true, RequiresPredecessors=false`。
- 移除其专属控制器方法与 `POST /migrate/20260620` 路由，改由启动自动执行 + 通用端点覆盖。
- 该改造**不在本分支**做；本分支只交付框架（注册表为空，用测试里的假迁移验证）。

## 影响面 / 交付顺序

- 新增 `internal/migrate`：`registry.go`、`runner.go`、`record.go`（或合并）+ 单测。
- `cmd/server` 启动序列新增一行 `migrate.RunAuto`。
- `internal/controller/migrate` + `registerMigrate` 增加 registry / run/:version / run-pending 端点。
- 旧迁移与端点零改动。
- 顺序：**本分支实现并 review → 合并进 `feat-paid-content-flag` → 在该分支把 20260620 注册为
  `Version=20260620000000/Auto` 并删其专属端点**。

## 风险 / 注意

- **乱序应用**：补更早版本时会在更晚版本之后执行；warning + `RequiresPredecessors` 兜底，
  对独立数据迁移无害。
- **不阻断启动**：自动迁移失败仅记日志，不挡服务（保可用性）。
- **记录与执行非原子**：靠幂等兜底（§3）。

## 验收

- 版本号唯一时正常；重复/非正版本号启动 panic（单测）。
- 全新库启动：auto 且前置满足者被执行并写入 `schema_migrations`；重启不重跑。
- `Auto=false` 启动不跑，`POST /migrate/run/:version` 可手动跑并记录。
- `RequiresPredecessors=true` 在前置未完成时不执行（自动与手动均拒绝），日志说明。
- 未完成且版本小于已完成最大版本者，启动记乱序 warning 且仍执行（单测/日志）。
- `GET /migrate/registry` 如实反映 completed/applied_at；旧迁移端点行为不变。

## review 决策记录

1. 自动迁移失败 → **只记日志、不阻断启动**。
2. 端点 `registry` + `run/:version` + `run-pending` **够用**。
3. 表名用社区惯例 **`schema_migrations`**。
4. **单实例**，不加 advisory lock（注释留多实例扩展点）。
5. 采用**时间戳 `int64` 版本 + 集合表**（用 `mapset`）；完整性由「差集每次跑干净」保证，
   去掉连续校验，保留去重 panic + 乱序 warning。
