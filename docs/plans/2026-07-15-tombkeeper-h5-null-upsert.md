---
title: "修复 tombkeeper H5 图片索引并建立数据库不变量"
issue: docs/issues/2026-07-15-tombkeeper-h5-null-upsert.md
status: done
areas: [tombkeeper, db, migrate]
updated: "2026-07-15"
---

# PLAN: 修复 tombkeeper H5 图片索引并建立数据库不变量

## 目标

解决 [issue](../issues/2026-07-15-tombkeeper-h5-null-upsert.md)：修正 H5 成功空结果写成 JSON
`null` 的源头；用自动 migration 清理已有非法数据，并由 PostgreSQL constraint 永久维护
`view_pics` 的结构不变量。`mergeH5ImageIDsSQL` 只处理满足 schema 的合法数据，不承担 migration
失败或旧数据未清理时的兼容职责。

## 关键决策

### 1. importer 负责产生合法的成功结果

`TimelineImporter.resolveMissingH5ImageIDs` 在 `GetReppic` 成功后总是保存非 nil slice：空结果规范化为
`[]string{}`，非空结果复制后保存。这样 GORM JSON serializer 输出 JSON `[]`，不会再制造新
`null`。请求失败仍不创建 key，保持三态语义：key 缺失表示以后重试，空数组表示成功但无图，非空数组
表示已取得 pic id。

单元测试不用 `assert.Empty`，而是同时断言 slice 非 nil，并用 `encoding/json` 检查待写模型中该
value 的 JSON 字面值为 `[]`。PostgreSQL 集成测试再原生读取一次正常 `UpsertPost` 的结果，锁定
“importer → Go 模型 → GORM serializer → jsonb”边界。

### 2. migration 清理历史数据并建立数据库不变量

新增自动 migration `20260715000000`（`tombkeeper-h5-image-index-invariant`，
`RequiresPredecessors=true`），在一个事务中完成：

1. SQL NULL、顶层 JSON null 或其他非 object 的 `view_pics` 归一化为 `{}`；
2. 对 object 逐 key 重建：value 是 array 时原样保留，JSON null、string、number、boolean、object 等
   非数组 value 统一改为 `[]`；
3. 给 `view_pics` 设置 `DEFAULT '{}'::jsonb` 与 `NOT NULL`；
4. 增加具名 CHECK constraint：顶层必须是 JSON object，且不存在 value 类型不是 array 的 key。

constraint 使用 PostgreSQL jsonpath 表达“没有非数组 value”，不在 CHECK 中引入子查询；精确表达式用
与生产一致的 PostgreSQL `16.3-alpine` 集成测试验证。本次不在 `Post.H5ImageIDsByURL` 的 GORM tag
声明 `NOT NULL` 或默认值：启动顺序是 `AutoMigrate` 先于注册 migration，提前同步 tag 可能在历史 SQL
NULL 尚未清理时直接令 `AutoMigrate` 失败，使 migration 没有运行机会。列清理、default、NOT NULL 与
嵌套 CHECK 全部只由注册 migration 负责。

migration 以该具名 constraint 是否存在作为幂等 guard。清理、列约束和 CHECK 都在同一事务中，因此
constraint 已存在即可证明 migration body 已整体提交；若 body 已提交但 `schema_migrations` 记账失败，
下次启动 no-op 后补记，不重复改写数据。fresh schema 由 `AutoMigrate` 先建表，再执行同一 migration，
空表也能安全建立约束。

不增加手工生产 SQL，不在 migration 外扫描或改写生产数据。

### 3. merge SQL 信任 schema，不兼容非法类型

`mergeH5ImageIDsSQL` 保持数组域内的单调合并：

1. incoming 数组非空时采用 incoming；
2. incoming 为空且 existing key 已存在时保留 existing；
3. existing 不含 key 时采用 incoming 空数组。

数据库 constraint 保证 existing 与 incoming 的 value 都是数组，因此 merge SQL 可以直接调用
`jsonb_array_length`，不增加 `jsonb_typeof`、异常值归一化或 migration 失败兜底。`in_timeline` 继续用
`old OR new`，只允许 `false → true`。

migration 失败仍属于 migration 自身的失败，不把这一状态转化成 tombkeeper 写路径的兼容分支。

### 4. 真实 PostgreSQL 测试覆盖故障、迁移与约束

任何实现修改之前，先在真实 PostgreSQL 写 importer→DB 回归测试：用成功空结果的 fake requester 配
真实 `DBService` 导入同一页面两次。旧实现第一次把 value 持久化为 JSON `null`，第二次从 DB 读到已
存在的 key 后不重抓，并在 `UpsertPost` 稳定触发 SQLSTATE 22023；测试同时从 raw jsonb 与
`ImportStats.EntriesFailed` 锁定这条故障链，并把第一次导入后读回的 `Post` 直接交给
`DBService.UpsertPost`，让断言输出保留原始 SQLSTATE 22023，而不只依赖 importer 日志。记录红灯后
才允许修改 importer 或添加 migration。

随后写 migration 集成测试，并在完整 migration body 实现前用最小 no-op skeleton 保留旧
`{"url": null}`，证明 migration 后再次 upsert 的期望仍以 SQLSTATE 22023 红灯；再实现清理与约束，
使同一测试转绿。

迁移集成测试还覆盖：

- SQL NULL、顶层 JSON null／非 object 分别变为 `{}`；
- object 内 JSON null、string、number、boolean、object 分别变为 `[]`；合法空／非空数组原样保留；
- constraint 拒绝列 SQL NULL、顶层非 object 和任一非数组 value，接受 `{}`、空数组及非空数组；
- 已有合法行发生 ON CONFLICT 时，`UpsertPost` 传入 nil slice（`EXCLUDED.view_pics` value 为 JSON
  null）会被 CHECK 以 constraint violation 拒绝，merge SQL 不负责兼容，且 existing 行保持不变；
- migration 连跑两次，以及“body 已完成、applied record 未写入”后重跑，结果幂等；
- fresh schema 执行 migration 成功；
- migration 后真实 upsert 继续满足新非空优先、空不覆盖旧非空、`in_timeline` 只升不降。

所有数据库测试只连接显式传入的临时测试 DSN；实施时使用与生产 compose 一致的临时 PostgreSQL
`16.3-alpine` 容器，不连接生产库。

## 代码落点

- `pkg/routers/tombkeeper/import_test.go`：收紧成功空 H5 结果断言，区分 nil 与非 nil 空 slice，并验证
  JSON 表示。
- `pkg/routers/tombkeeper/import.go`：成功结果统一复制到非 nil slice；请求失败行为不变。
- `pkg/routers/tombkeeper/db_integration_test.go`：锁定 GORM 非 nil 空 slice 真实落库为 `[]`，保留既有
  合法数据域内的并发单调 upsert 测试；增加 importer→真实 DB 的生产故障链红灯。
- `internal/migrate/20260715000000.go`：事务清理、列级 NOT NULL/default、具名 CHECK constraint 与幂等
  guard。
- `internal/migrate/20260715000000_integration_test.go`：真实 PostgreSQL 迁移、约束、重试和 fresh schema
  回归测试。
- `pkg/routers/tombkeeper/example/README.md`：把 `view_pics` 的 JSON object/array 不变量与三态语义写入
  SSOT。

## 实施步骤（对应提交）

1. **真实 PostgreSQL 红灯先行**：在不修改任何实现的前提下，先增加 importer→真实 DB 集成测试，
   使用 PostgreSQL `16.3-alpine` 记录首次 JSON `null` 与第二次 SQLSTATE 22023；再收紧 importer 单元
   测试。红灯证据写入 lesson，测试暂不单独提交。
2. **修复 importer**：修改成功空结果的 slice 规范化，使同一真实 DB 测试与 GORM `[]` 落库断言转绿，
   连同第一步测试提交为一个 `fix(tombkeeper)` commit。
3. **迁移测试先行并建立约束**：写 PostgreSQL migration 回归测试，配最小 no-op skeleton 再记录旧
   null 未清理时的 22023 红灯；实现清理、NOT NULL/default、CHECK constraint 和幂等 guard，增加非法
   incoming ON CONFLICT 被拒且 existing 不变的测试，提交一个 `fix(migrate)` commit。
4. **完成文档、验证与评审**：创建 lesson，更新 H5 SSOT，保持 issue 为 open、plan 为
   in-progress；运行目标测试与 `just lint`，再执行 `/code-review` 的 Standards + Spec 独立实现评审
   并处理结论。评审通过后关闭 issue，并在同一个 `docs(tombkeeper)` 收尾提交更新
   `docs/PROGRESS.md`；plan 仅在 squash 合并时改为 done。

## 测试

- 红灯 1（任何实现修改前）：设置临时 `TOMBKEEPER_TEST_DATABASE_URL`，运行新增的
  importer→`DBService` 集成测试，修复前确认首次 raw value 为 JSON `null`，第二次导入不重抓且
  upsert 报 SQLSTATE 22023；同时运行收紧后的 importer 单元测试，确认 nil slice／JSON `null`。
- 红灯 2：运行新增的 migration 集成测试；no-op skeleton 下旧 `{"url": null}` 未被清理，后续 upsert
  稳定报 SQLSTATE 22023。
- 绿灯：使用同一临时 PostgreSQL `16.3-alpine`，运行
  `go test ./pkg/routers/tombkeeper/... ./internal/migrate/...`，覆盖 importer、GORM 落库、migration、
  constraint、非法 incoming ON CONFLICT 拒绝且 existing 不变、合法数据域 upsert、既有 tombkeeper
  golden 与 migration registry。
- 运行 `just lint`；不默认运行全量测试。
- 实现结束后运行 `/code-review`，由独立 reviewer 并行检查 Standards 与 Spec；findings 要么当前修复，
  要么创建后续 issue，不写独立 findings 文件。

## 待更新文档

- [x] `pkg/routers/tombkeeper/example/README.md`：明确 `view_pics` 的 object/array 数据库不变量与 H5
      三态语义。
- [x] `docs/lessons/2026-07-15-tombkeeper-h5-null-upsert.md`：记录红灯复现、数据约束归属、迁移幂等与
      验证结果。
- [x] `docs/issues/2026-07-15-tombkeeper-h5-null-upsert.md`：实现评审通过后改为 `closed`。
- [x] `docs/plans/2026-07-15-tombkeeper-h5-null-upsert.md`：开工时改 `in-progress`，squash 合并后改
      `done`。
- [x] `docs/PROGRESS.md`：在完成分支的同一提交记录修复、migration 与验证结论。
- [x] `docs/ARCHITECTURE.md`：无需修改；模块边界和持久化模型的概念结构未变。
- [x] `docs/OPS.md`：无需修改；本次不改变标准部署方式，不记录 migration 内部约束细节。
- [x] `docs/TODO.md`：当前未发现计划外问题；若实施中出现，按约定即时补记或创建后续 issue。

## 后续项

无。migration 或 constraint 失败留在 migration 模块处理，不在其他模块增加兼容路径。
