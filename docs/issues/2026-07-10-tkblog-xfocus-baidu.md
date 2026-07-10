---
title: "引入 tombkeeper.io xfocus / baidu 博文（解析·落库·归档）"
kind: feature
status: open
priority: medium
areas: [tombkeeper, controller, migrate, db]
plan: docs/plans/2026-07-10-tkblog-xfocus-baidu.md
related: [pkg/routers/tombkeeper, internal/controller/archive]
updated: "2026-07-10"
---

# SPEC: tombkeeper xfocus / baidu 博文

## 背景

已经引入了 tombkeeper.io 的**微博**博文（`pkg/routers/tombkeeper`，含转发/图片/嵌入）。
tombkeeper.io 上另有两批**博客文章**：

- `https://tombkeeper.io/xfocus` —— 早年 blog.xfocus.net 的文章（约 73 篇 / 15 页）。
- `https://tombkeeper.io/baidu` —— 早年 hi.baidu.com 的博客（约 164 篇 / 33 页）。

它们比微博简单得多：**纯文本正文，没有转发、没有图片、没有嵌入**。内容已定型（历史存档，
不再新增），因此不需要 cron 增量。

## 目标

把 xfocus / baidu 两批博文**解析、落库**，并提供**单篇归档页**（返回 HTML），与其他源一致。

- 两者同为 tk 的博客文章、页面模板完全相同，**共用一张表**，用一个 `category` 字段标明出处
  （`xfocus` / `baidu`）。
- 抓取用**伪 job**（后台 goroutine，非 cron）：触发即**全量抓取**（每次都是全量、幂等 upsert）。
  两个源各一个触发入口。
- 页数在第一页即可读到（和微博一致）。
- 落库两类链接：**粉丝站链接** = `https://tombkeeper.io/{category}/{id}`（页面内的「阅读全文」
  链接）；**原文链接** = 页面内的 **Wayback Machine** URL（`web.archive.org/...`）。
- **归档功能**：给定单篇 URL 返回 HTML，复用 `/api/v1/archive/:url` 分派机制。

## 实测确认（真实请求 tombkeeper.io，2026-07-10）

- 站点为 Next.js App Router SSR；结构化数据在 `self.__next_f.push([1,"…"])` 的 RSC flight
  分片里，字段干净：`{"id","category_slug","title","created_at":"$D…ISO","url"(wayback),"content"}`。
  —— 与微博 `extract.go` 同一套 flight 机制，但**无 retweet_id / pics / url_info**，更简单。
- **正文在列表页即完整存在**（不截断）：抓列表页即可拿到全部字段，**无需再抓详情页**。
- 每页 5 篇；总页数在第 1 页可读：flight 里 `"totalPages":N`，或分页器 `?page=N` 最大值，
  或页首「共 N 篇文章」计数。翻页 URL：`/{category}?page=N`（第 1 页为裸 `/{category}`）。
- xfocus 与 baidu **页面模板、类名完全一致**，仅数据不同（slug、wayback 内的原站域名、总数）
  —— 一套按 category 参数化的解析即可覆盖两者。
- 正文为 `whitespace-pre-wrap` 的纯文本（`\n` 有意义），**无 `<img>`、无转发、无嵌入**。
- flight 里 `content` 有时是 `$ref`（如 `"$16"`）引用先前行 —— 解析需做 `$ref` 解引用
  （微博 `extract.go` 已有此能力）；`created_at` 带 `$D` 前缀（Flight 的 Date 标记，剥掉即 ISO）。

## 验收

- 触发 xfocus / baidu 的伪 job，能把各自全部博文入库；重复触发幂等，不产生重复。
- 每条记录含 category、id、title、created_at、正文、wayback 原文链接；粉丝站链接可由
  `category + id` 推导。
- `GET /api/v1/archive/<粉丝站链接>` 返回该篇 HTML（含正文 + 原文/粉丝站链接页脚）。
- 新增测试：flight 解析、正文→markdown、归档链接匹配、抓取 happy-path（stub）。

## 不做什么

- **不做 cron 增量**（内容已定型）。
- **不做断点续传 / 进度查询**（全量 + 幂等 + 篇数少，崩了重触发即可）。
- **不抓详情页**（列表页已含全文）。
- **不处理图片/转发/嵌入**（源本身没有）。
- RSS/Atom 订阅源：**本次不做**（内容定型、订阅价值低）；如需，见 plan「后续项」，由作者在放行时决定是否纳入。
