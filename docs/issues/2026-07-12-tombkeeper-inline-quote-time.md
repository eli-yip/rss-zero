---
title: "tombkeeper 内嵌微博正文显示目标发布时间"
kind: feature
status: closed
priority: medium
areas: [tombkeeper, rss, render]
plan: docs/plans/2026-07-12-tombkeeper-inline-quote-time.md
related: [pkg/routers/tombkeeper/render_markdown.go, pkg/routers/tombkeeper/shortlink_test.go]
updated: "2026-07-12"
---

## 问题

tombkeeper 的显式转发引用块会在末尾显示被转发微博的北京时间，但正文中「微博正文」短链展开成的
内嵌引用块只显示作者与正文。两种方式都在引用一条已装配的目标微博，却对目标发布时间采用不同的
展示规则，读者无法从内嵌引用判断原文所处的时间语境。

## 目标

- 正文中的「微博正文」链接成功展开为内嵌引用时，在引用块末尾显示目标微博的北京时间。
- 时间格式与显式转发完全一致，并继续由纯 Markdown renderer 确定性生成。
- 保持一层引用限制以及目标缺失时退回普通链接的现有行为。

## 验收

- 目标 `PublishedAt` 有效时，每个「微博正文 N」引用块末尾显示格式为
  `YYYY 年 MM 月 DD 日 HH:mm` 的北京时间。
- 目标 `PublishedAt` 为零值时不显示伪造时间。
- 显式转发的时间展示与内嵌引用使用同一格式化规则，原有行为不回退。
- 回归测试在修复前失败、修复后通过，并覆盖有效时间与零值时间。

## 不做什么

- 不改变 Atom entry 自身的 `published` / `updated` 时间。
- 不改变抓取、数据库 schema、历史数据或引用依赖装配范围。
- 不展开第二层引用，不调整引用块的其他排版。
