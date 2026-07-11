---
title: "tombkeeper 事实入库与纯 Markdown 渲染重写复盘"
plan: docs/plans/2026-07-11-tombkeeper-fact-rendering.md
status: draft
areas: [tombkeeper, rss, controller, migrate, db, render]
updated: "2026-07-11"
---

# tombkeeper 事实入库与纯 Markdown 渲染重写复盘

实施中记录非显然的约束、返工点与验证结果；完成后整理为连贯复盘。

## 过程记录

- 开工前先把领域语言固定在 `CONTEXT.md`：时间线条目、内嵌博文、图片资产、H5 图片索引、
  内容快照。避免实现重新引入 `Facts` / `Object` / `RenderDataset` 等处理步骤或泛化命名。
- `AutoMigrate` 先于注册迁移执行，因此新模型不能复用旧的 `retweet_id text` 列；使用
  `retweet_post_id bigint`，再由破坏性迁移删除旧列，避免空字符串在启动阶段被强制转 bigint。
- migration 的事务提交与 `schema_migrations` 记账之间存在进程退出窗口。以 legacy
  `text_markdown` 列是否仍存在作为迁移守卫，可保证重试不会再次清空已经导入的新结构化博文。
- H5 图片索引必须区分三态：不存在表示尚未求取、空数组表示求取成功但没有图片、非空数组表示
  已解析到 pic id。只有网络/解析错误保持不存在并允许下次重试。
- `in_timeline` 与 H5 索引都采用单调合并：前者只允许 false→true，后者按 absent < empty <
  nonempty 合并。用 PostgreSQL 原生并发 upsert 测试确认旧详情页写入不会覆盖较新的时间线事实。
- 除包级 fixture / golden / build 外，用临时 PostgreSQL 17 容器跑过并发 upsert 与破坏性迁移重试
  集成测试；两项均通过，未对用户数据库执行迁移。
- 实现评审发现图片资产也有单调性要求：实时抓取与 history 可能同时处理同一 pic id，普通 `Save`
  会让后到的失败覆盖已成功 OSS 资产。改为 PostgreSQL 原子 upsert，规定成功不可退化，并加入并发测试。
- `ContentLoader` 的“两阶段”必须体现在代码结构上：先登记全部 roots，再收集引用；否则前一个 root
  引用后一个 root 时，会从数据库重读并覆盖调用方传入的 root。
- fresh schema 的完整 migration registry 测试暴露出旧零字节图片迁移会在无图片时仍初始化 Minio，
  从而阻塞带 predecessor 的新迁移。先查询候选行、空集直接返回后，fresh DB 可完成全部自动迁移。
- 最终 PostgreSQL 集成验证覆盖：博文/H5/图片资产并发单调 upsert、DDL 事务失败回滚、全部旧列删除、
  迁移记账窗口重试，以及 fresh schema 完整自动迁移；均通过。
- 初版 Importer 虽然对调用方只有一个深 interface，内部却用 `sources + entryIDs` 两张平行 map 表达
  同一个候选状态，并在逐引用 `GetPost` 后再次批量读库。最终把依赖可用性拆成具名阶段：
  `postsProvidedByPage` 包含时间线帖和内嵌帖（包括非 tk 的 `retweet_weibo`），`storedPostsByID` 表示
  历史归档，`referencesToFetch` 只保留两处都不存在的直接依赖；最后汇合为 `postsToImport`。全程只
  读取一次数据库，数据库读取错误发生时也不会先发无意义的网络请求。
- 初版 `enrichPost` 把 H5 解析、OSS/图片资产写入合成一个前置步骤，任一图片错误都会阻止博文事实
  入库。新顺序先解析 H5、upsert 博文，再 best-effort 补图片资产；资源故障不再丢失正文、链接或
  H5 索引。`referencePostIDs` 同时供摄取与读取装配使用，直接依赖定义只保留一份。
- 在临时 PostgreSQL 17 完成 fresh migration 后，使用真实 tombkeeper page 1 验证页面提取及
  `in_timeline=true` 写入；测试事务回滚，未触碰用户数据库。
- `ImportStats.EntriesSaved` 只用于单次任务可观测性：它基于导入开始时的批量读库快照，live/history
  并发命中同一帖时可能重复观察。日志明确使用 `observed_*`，不为非全局计数引入多语句事务。
