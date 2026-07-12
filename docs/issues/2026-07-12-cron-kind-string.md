---
title: "cron 任务类型从 int(iota) 改为字符串 Kind"
kind: refactor
status: open
priority: medium
areas: [cron, db, controller, migrate]
plan: docs/plans/2026-07-12-cron-kind-string.md  # TBD，待 Issue 1 落地后补写
depends_on:
  - docs/issues/2026-07-12-cron-source-registry.md
related:
  - pkg/cron/db/definition.go
  - internal/controller/job/task.go
updated: "2026-07-12"
---

> **plan 待写**：本 issue 依赖 [Issue 1（来源注册表）](2026-07-12-cron-source-registry.md) 先
> 落地。`plan:` 字段指向的 `docs/plans/2026-07-12-cron-kind-string.md` 目前是占位路径，等
> Issue 1 合并后再补写方案。

## 问题

`CronTask.Type` 是**位置相关的 `iota`**：`TypeZsxq/TypeZhihu/TypeXiaobot/TypeGitHub` 依次取
0/1/2/3（`pkg/cron/db/definition.go:34` 起的 const 块），并以 `int` 列
（`definition.go:17`）持久化到 `cron_tasks.type`。

这是个 footgun：**重排、插入或删除这个 const 块里的任何一项，都会静默改写已存历史行的语义** ——
DB 里存着 `2` 的行本来是 xiaobot，若有人在 zsxq 前面插入一个新来源，`2` 就悄悄变成了别的源，没有
任何编译错误或运行报错来提示。int 值本身不自描述，脱离这段 const 块无法解读。

而 API 层本来就说字符串（请求里的 `task_type` 是 "zsxq"/"zhihu"/… ，`internal/controller/job/task.go`
的 `AddTask` 注释与 `TaskTypeStrToInt` / `TaskTypeIntToStr` 就是在 API 字符串与这个 int 之间做
桥接）。也就是说 int 只在 DB 与内部 switch 里存在，两头都要转换，中间层的 int 纯属多余的间接。

## 目标

把持久化改成稳定的字符串 `Kind`（`"zsxq"` / `"zhihu"` / `"xiaobot"` / `"github"`），让 DB 行
自描述、不受 const 顺序影响；[Issue 1](2026-07-12-cron-source-registry.md) 的来源注册表直接以
`Kind` 字符串为键，`TypeStrToInt` / `TypeIntToStr`（Issue 1 已把它们塌成注册表查询）随之彻底
消失——API 字符串、注册表键、DB 列三者统一为同一个字符串，不再有任何转换层。

## 验收

- `cron_tasks` 用 `kind`（字符串）列存来源；`CronTask` 结构体字段从 `Type int` 改为
  `Kind string`。
- 有一条自动 migration 把现有行的 int 回填成对应字符串（0→"zsxq"、1→"zhihu"、2→"xiaobot"、
  3→"github"），并处理旧 `type` 列的去留（新增 `kind` 后回填、再删 `type`）。
- 代码里不再有基于 int 来源枚举的 switch 或转换；来源注册表以 `Kind` 为键
  （`specByKind` 取代 `specByType`）。
- `iota` const 块（`TypeZsxq…`）删除。

## 迁移风险

低。`cron_tasks` 是用户手动增删的动态任务定义，行数极少（个位数量级），一次性回填 int→string 无
性能或数据量顾虑；且回填是确定映射，无歧义。

## 不做什么

- 不改 API 契约：请求/响应里的 `task_type` 本就是字符串，保持不变（本 issue 只是让 DB 与内部也用
  同一字符串，去掉中间的 int）。
- 不动 `cron_service_job_id` 列（那是 [Issue 2](2026-07-12-cron-drop-jobid-column.md)）。
- 不改各源 `BuildCrawlFunc` 与 gocron 封装。
- 不与 Issue 1 / Issue 2 合并成一次改动：三件事各自独立、各自可回滚，分开做以缩小每次 diff 与迁移
  面。本 issue 必须在 Issue 1 的注册表落地后进行，届时「以 Kind 为键」是顺势替换而非新造抽象。
