---
title: "tombkeeper 内嵌微博正文时间展示"
issue: docs/issues/2026-07-12-tombkeeper-inline-quote-time.md
status: in-progress
areas: [tombkeeper, rss, render]
updated: "2026-07-12"
---

# PLAN: tombkeeper 内嵌微博正文时间展示

## 目标

解决 [issue](../issues/2026-07-12-tombkeeper-inline-quote-time.md)：让正文中成功展开的「微博正文」
引用与显式转发一样，在引用内容末尾显示目标微博的北京时间，补足原文的时间语境。

## 关键决策

- 在 `renderSnapshotLinks` 已取得目标 `Post` 后，把时间行加入该目标的引用正文；不修改抓取、
  `ContentSnapshot` 或数据库，因为 `Post.PublishedAt` 已是渲染所需的完整事实。
- 复用现有 `retweetTimeLine` 的 UTC+8 转换与格式，避免两种引用产生不同时间文案；若目标时间为
  零值则不追加时间，和显式转发当前的防御行为一致。
- 保持时间位于引用块正文末尾，而不是标题行；这样两种引用的视觉层级一致，也不改变引用编号、
  作者标题和归档链接。

## 方案

在 `pkg/routers/tombkeeper/render_markdown.go` 提取或复用一个为引用正文按需追加发布时间的窄 helper，
让显式转发和「微博正文」内嵌引用都经过同一逻辑。renderer 仍只读取传入的自包含快照，不增加 I/O、
全局时间或递归深度。

在 `shortlink_test.go` 构造带确定时区时间的目标帖，锁定内嵌引用块末尾的北京时间；另以零值时间
验证不会输出公元元年或其他伪造文案。现有转发 fixture 测试继续证明显式转发行为未回退。

## 实施步骤（对应提交）

1. 先增加内嵌引用有效时间与零值时间的回归测试，确认有效时间用例在修改前失败。
2. 让显式转发与内嵌引用共用按需追加北京时间的渲染逻辑，使回归测试转绿。
3. 更新渲染规则文档和 lesson，运行 tombkeeper 包测试及 `just lint`，再做实现评审。
4. 处理评审结论，关闭 issue；完成分支时在同一文档提交中更新 PROGRESS。

## 测试

- `go test ./pkg/routers/tombkeeper/...`
  - 「微博正文」目标有时间：引用块末尾出现转换后的北京时间。
  - 「微博正文」目标为零值时间：引用块不出现时间行。
  - 现有显式转发、单层引用及 Atom golden 测试保持通过。
- `just lint`

遵循仓库约定，不运行未触及包的全量测试。

## 待更新文档

- [x] `pkg/routers/tombkeeper/example/README.md`：把「微博正文」引用也显示目标北京时间写入 SSOT。
- [x] `docs/lessons/2026-07-12-tombkeeper-inline-quote-time.md`：记录共享引用时间逻辑与验证结果。
- [x] `docs/issues/2026-07-12-tombkeeper-inline-quote-time.md`：实现完成后关闭。
- [x] `docs/PROGRESS.md`：分支完成时记录用户可见变化与验证结论。

`docs/ARCHITECTURE.md` 与 `docs/OPS.md` 无需更新：本改动不改变模块边界、数据流、schema 或运维流程。

## 后续项

无。若实现中发现其他引用展示差异，另开 issue，不扩大本次范围。
