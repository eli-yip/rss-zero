---
title: "tombkeeper H5 图片索引违反 JSON 数组不变量"
kind: bug
status: closed
priority: high
areas: [tombkeeper, db, migrate]
plan: docs/plans/2026-07-15-tombkeeper-h5-null-upsert.md
related: [pkg/routers/tombkeeper/import.go, pkg/routers/tombkeeper/db.go, internal/migrate]
updated: "2026-07-15"
---

## 问题

线上版本 26.7.7 从 2026-07-14 13:00（Asia/Shanghai）起，每小时 tombkeeper crawl 都在
微博 `5320541594714302` 入库时失败，PostgreSQL 报
`cannot get array length of a scalar (SQLSTATE 22023)`。该行的 `view_pics` 已包含：

```json
{
  "https://photo.weibo.com/h5/repost/reppic_id/1022:230796bdb96ac84113498a7b46c906e7a4358b": null
}
```

`TimelineImporter.resolveMissingH5ImageIDs` 在 H5 请求成功但结果为空时保存 nil slice，GORM JSON
serializer 将它写为 JSON `null`；后续导入把这个 key 视为已经解析，不再请求。`UpsertPost` 的
`mergeH5ImageIDsSQL` 又假定每个 value 都是数组，直接调用 `jsonb_array_length`，因此已有异常行会让
后续每次 upsert 稳定失败。现有 fake DB 测试不能区分 nil slice 与非 nil 空数组，Postgres 集成测试
也只覆盖了序列化为 `[]` 的非 nil 空 slice。

## 目标

- H5 请求成功但没有图片时，持久化的索引 value 必须是 JSON 数组 `[]`，不能是 `null`。
- 自动 migration 将已有 JSON `null` 或其他不符合模型的 `view_pics` 数据归一化，并建立数据库约束，
  使非法数据不能再次入库。
- `mergeH5ImageIDsSQL` 只处理数据库约束保证的合法数组，不承担 migration 失败或历史坏数据的兼容。
- 保持单调合并语义：新非空图片 id 优先；新空结果不覆盖旧非空结果；`in_timeline` 只允许
  `false → true`。

## 验收

- 修复前必挂的真实 PostgreSQL 回归测试先复现旧行含 `{"url": null}` 时的 SQLSTATE 22023；实现后
  自动 migration 把该 value 及其他非数组 value 归一化为 `[]`。
- migration 把 `view_pics` 约束为非 NULL JSON object，且每个 value 都必须是 JSON array；测试证明
  PostgreSQL 会拒绝违反任一不变量的新写入。
- migration 在 fresh schema、已有合法数据和“migration body 已完成但 applied record 未写入”的重试
  场景中均安全、幂等。
- importer 测试明确区分 nil 与非 nil 空 slice，并证明成功空结果序列化语义为 `[]`。
- 迁移完成后的真实 PostgreSQL upsert 测试继续覆盖非空新图片 ID 优先、空结果不覆盖已有非空结果，
  以及 `in_timeline` 只能从 false 变为 true。
- tombkeeper 目标测试与 `just lint` 通过。

## 不做什么

- 不手工连接或修改生产数据库；历史数据只由随版本发布的注册 migration 自动清理。
- 不改变 tombkeeper 抓取范围、H5 请求重试策略、RSS 渲染或图片归档流程。
- 不让 importer 或 merge SQL 为 migration 失败兜底；migration 未完成属于部署失败，由现有 migration
  告警与状态检查处理。
- 不把本次约束扩展成通用 JSON schema/migration 框架。
