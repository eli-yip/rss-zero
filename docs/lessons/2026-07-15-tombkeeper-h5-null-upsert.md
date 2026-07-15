---
title: "tombkeeper H5 图片索引 null 故障复盘"
plan: docs/plans/2026-07-15-tombkeeper-h5-null-upsert.md
status: done
areas: [tombkeeper, db, migrate]
updated: "2026-07-15"
---

# tombkeeper H5 图片索引 null 故障复盘

- PostgreSQL 16.3 红灯完整复现了线上链路：H5 成功空响应第一次经 GORM 写成
  `{"url":null}`；下一次 importer 看到 key 已存在便不重抓，`mergeH5ImageIDsSQL` 对该 value 调
  `jsonb_array_length`，稳定报 SQLSTATE 22023。
- nil 不只来自 `resolveMissingH5ImageIDs`。首次把空响应改为非 nil slice 后，真实 DB 测试发现
  `cloneH5ImageIDs` 又会把读回的空数组克隆成 nil，第二次 upsert 仍失败。两处统一以非 nil slice
  复制后，单元测试和真实 GORM→jsonb 路径都稳定持久化 `[]`。
- 历史非法数据由 `20260715000000` 自动 migration 负责：顶层 SQL NULL、JSON null 和其他非 object
  变为 `{}`；object 内非数组 value 变为 `[]`；合法空／非空数组原样保留。清理、default、NOT NULL
  与具名 CHECK constraint 在同一事务中完成，constraint 同时作为“body 已提交但迁移记账失败”的
  幂等 guard。
- PostgreSQL jsonpath 默认 lax 模式会自动展开数组；最初的 `$.*` CHECK 把合法 `["pic"]` 的元素
  当成直接 value，错误地拒绝合法写入。改为 `strict $.*` 后，类型检查才准确落在 object 的直接
  value 上。归一化扫描也使用同一 strict 路径，避免无意义重写合法非空数组。
- constraint 在 PostgreSQL 16.3 的 ON CONFLICT 路径中先拒绝非法 `EXCLUDED.view_pics`，返回
  SQLSTATE 23514，existing 行保持不变；merge SQL 因而只处理 schema 允许的数组，不为 migration
  或非法调用方兜底。
- 使用临时 `postgres:16.3-alpine` 串行运行
  `go test -p 1 ./pkg/routers/tombkeeper/... ./internal/migrate/...` 全绿；`just lint` 全绿。未连接或修改
  生产数据库。
- 双轴实现评审最初指出两处重复 helper 与一项“migration 后未重测合法域单调 upsert”的 Spec 缺口；
  提取非 nil 图片 id clone 和 PostgreSQL 测试 DB setup helper，并补 migration 后的非空优先、空不
  覆盖非空、`in_timeline` 只升不降测试后，Standards 与 Spec 复核均 PASS。
