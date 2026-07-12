---
title: "tombkeeper 内嵌微博正文时间展示复盘"
plan: docs/plans/2026-07-12-tombkeeper-inline-quote-time.md
status: draft
areas: [tombkeeper, rss, render]
updated: "2026-07-12"
---

# tombkeeper 内嵌微博正文时间展示复盘

- 显式转发与「微博正文」内嵌引用原先分别拼装引用正文，导致同一种目标博文事实采用了不同的时间
  展示策略。把“渲染一层目标正文，并在时间非零时追加北京时间”收进 `snapshotQuoteBody` 后，两条
  路径共享同一规则，同时没有改变递归深度或内容快照边界。
- 回归测试从公开纯函数 `RenderMarkdown` 观察完整 Markdown，并以对应引用块的末尾为边界断言；
  这比全局搜索时间字符串更能防止时间误落进根帖或其他引用块。
- TDD 红灯确认旧输出止于目标正文；有效时间用例转绿后再加入零值用例，锁定不会生成公元元年时间。
- `go test ./pkg/routers/tombkeeper/...` 与 `just lint` 通过。双轴实现评审中 Spec 零发现；Standards
  只要求按计划完成 issue 与 PROGRESS 收尾，已在同一文档提交中处理。
