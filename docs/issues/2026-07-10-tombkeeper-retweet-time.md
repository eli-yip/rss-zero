---
title: "转发微博在引用块末尾注明原博时间"
kind: feature
status: open
priority: low
areas: [tombkeeper, render]
plan: docs/plans/2026-07-10-tombkeeper-retweet-time.md
related: [pkg/routers/tombkeeper/parse.go]
updated: "2026-07-10"
---

## 问题

转发（retweet）微博渲染时，被转发原文内联在引用块里
（[`parse.go:82`](../../pkg/routers/tombkeeper/parse.go) 的
`quoteBlock("转发 @"+orig.ScreenName, quote)`），但**原博的发布时间没有任何呈现**：
顶层帖的时间进了 RSS 的 `<updated>`／`published`，读者能看到；被转发原文的
`created_at` 已在手（`orig.CreatedAt`），却既不进 feed 元数据、也不在正文里显示。
于是读者看到「转发 XXX」却无从判断原博是哪天发的（转发原文常是旧内容）。

## 目标

在被转发原文的引用块**末尾追加一行**，注明该原博的发布时间。形如：

```
> 转发 @XXX
>
> ……原文内容……
>
> 2026 年 06 月 08 日 08:55
```

- 时间取 `orig.CreatedAt`（UTC instant），按 **北京时间**（`config.C.BJT`，全项目
  人可读微博时间的既定时区）格式化 —— 原文 `$D2026-06-08T00:55:15.000Z` → `08:55`。
- 追加行进入 `text_markdown` 存库，RSS 与归档页都随之呈现。

## 验收

- 转发帖的引用块末尾多出一行原博时间；非转发帖不受影响。
- 时区正确（Asia/Shanghai）。
- 补一条渲染快照/单测的回归用例（转发夹具，含 `created_at`）。

## 不做什么

- 不改顶层帖的时间呈现（已在 RSS 元数据里）。
- 不给 `微博正文 N` 内联引用块加时间 —— 除非评审认为一致性上也该加（**待 PLAN 确认**）。
- 不动 `created_at` 的解析/存储，只加渲染时的格式化输出。
