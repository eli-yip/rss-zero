# 文档与开发流程约定

本项目如何跟踪工作。三类工作文档 + 一组参考文档，扁平目录，彼此用相对路径互链。
范围以各 issue 为准；跨版本的产品/运维事实见参考文档。

## 目录布局

```
docs/
  PROGRESS.md      — running log，最新在上（发布/上线记录的事实来源）
  ARCHITECTURE.md  — 代码地图（模块、数据流、RSS 出口管线、存储）
  OPS.md           — 运维手册（构建、发版、部署、迁移、回填、告警）
  TODO.md          — 落在当前 PLAN 之外的后续项（技术债、延后修复、临时想法）
  CONVENTIONS.md   — 本文件
  issues/          — 一个问题 / 任务 / bug 一个文件（扁平）
  plans/           — 一份实现方案一个文件（扁平）
  lessons/         — 一份实施复盘/经验一个文件（扁平）
```

文件名：`YYYY-MM-DD-<slug>.md`。**状态写在 frontmatter，不写进路径** —— 文档一经创建
路径终身不变，入链永不失效。用状态检索工作，而不是靠目录：`just issues open` /
`just plans in-progress` / `just lessons draft`（也支持 `closed` `done` `wontfix` 等）。

> 历史文档（2026-07 本次迁移之前）不含 frontmatter，其状态见正文 `状态：` 行与
> [PROGRESS.md](PROGRESS.md)；新文档一律带 frontmatter。

## 工作流

```
issue（目标 / 问题：做什么、为什么）
  └─ plan（方案：怎么做；关键决策就记在这里）
       └─ lesson（实施中/后的经验与复盘）
       └─ 后续 issue(s)   ← 实施中发现的新问题，即时开成新 issue
```

- **issue** 说清 _做什么 & 为什么_。**每个 issue 先有 plan 再写代码 —— 没有 issue 直接进实现。**
- **plan** 说清 _怎么做_，**是关键决策的落点**（不另开决策文档），也是「开工前评审」的对象。
  plan 随工作量伸缩：琐碎修复几行即可（目标 · 改动 · 测试 · 动了哪些文档）；较大工作用下面的完整骨架。
- **lesson** 记录实施过程中的坑与经验；PLAN 完成后把它整理成一篇连贯的复盘。

## frontmatter schema

字段名保持英文（供 `just` 检索），值可中英混排。`status` 取值固定用英文以便检索。

**issue**

```yaml
title: "简短标题"
kind: bug | feature | refactor | perf | ops | tech-debt | docs
status: open | closed | wontfix
priority: high | medium | low
areas: [zhihu, zsxq, tombkeeper, rss, controller, migrate, ...]  # 见下
plan: docs/plans/YYYY-MM-DD-slug.md      # 必填 —— 每个 issue 先有 plan
depends_on: [docs/issues/...]            # 选填 —— 有未闭合依赖即为 blocked
follow_ups: [docs/issues/...]            # 选填 —— 本 issue 派生出的 issue
related: [internal/..., pkg/...]         # 选填 —— 涉及的代码/文档
updated: "YYYY-MM-DD"
```

正文：一段问题陈述 · **目标** · **验收**（粗粒度，不写像素级细节）· **不做什么**。

**plan**

```yaml
title: "简短标题"
issue: docs/issues/YYYY-MM-DD-slug.md    # 本方案解决的 issue
status: draft | in-progress | done       # done = 已 squash 合并
areas: [...]
updated: "YYYY-MM-DD"
```

正文：**目标**（为什么，链到 issue）· **关键决策**（选择 + 理由）· **方案**（粗粒度）·
**步骤**（对应提交）· **测试**（本次新增什么，必填）· **待更新文档**（`- [ ]` 清单，实现评审逐条核）·
**后续项**。

**lesson**

```yaml
title: "简短标题"
plan: docs/plans/YYYY-MM-DD-slug.md
status: draft | done                     # done = PLAN 完成后已整理成连贯复盘
areas: [...]
updated: "YYYY-MM-DD"
```

正文：实施中踩的坑、返工点、非显然的决策与其代价、给后来者的提醒。

## areas

不是封闭词表（本项目是单一 Go module），取涉及的 Go 包/源区域即可：源站
`zhihu / xiaobot / github / zsxq / tombkeeper / endoflife / macked / weibo / douyu`，
基础设施 `rss`（出口管线）`controller` `migrate` `db` `redis` `render` `cron`
`notify` `file` `httputil`，以及 `docs` / `infra`。其收益是**按需只跑受影响的包的测试**：
`go test ./internal/rss/...`、`go test ./pkg/routers/tombkeeper/...`。

## 规则

- **测试不是可选项。** 每份 plan 都新增测试；每个修掉的 bug 都补一个「修前必挂、修后转绿」
  的回归测试。爬虫源另留 golden 快照（`testdata/*.atom`）证「新渲染 == 旧渲染」。
- **文档保鲜，强制执行。** 每份 plan 带一个 _待更新文档_ 清单（PROGRESS / ARCHITECTURE /
  OPS / TODO / 相关 issue）。实现评审逐条核对：列了却没改的文档会挡住合并。
  文档之间大量互链，是「本该改却没改」的绊线。
- **每份 plan 两轮独立评审，评审者 ≠ 作者。**（1）_开工前 plan 评审_：对着 issue 重读一遍 ——
  是否完整、决策是否站得住、有没有列出测试与文档更新。（2）_合并前实现评审_：对 diff 跑
  `/code-review`（Standards + Spec 两轴并行），结论要么当场修，要么开成后续 issue —— 不落单独文件。
- **合并。** squash 进 `master`、删分支。PROGRESS 追一条：关键改动 + 发版/上线结论。
- **语言。** 文档、注释用中文；commit message 用英文（Conventional Commits）。
