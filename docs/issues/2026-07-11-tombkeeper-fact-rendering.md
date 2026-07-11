---
title: "tombkeeper 只存事实并在读取时渲染 Markdown"
kind: refactor
status: open
priority: high
areas: [tombkeeper, rss, controller, migrate, db, render]
plan: docs/plans/2026-07-11-tombkeeper-fact-rendering.md
related: [pkg/routers/tombkeeper, internal/controller/tombkeeper, internal/controller/archive]
updated: "2026-07-11"
---

## 问题

当前 tombkeeper 抓取路径把源站事实获取、资源转存和展示策略都塞进
`Renderer.Render`：抓取时生成 `tombkeeper_post.text_markdown`，并把短链展开、图片失败提示、
转发／「微博正文」引用顺序、原博时间等业务规则烘焙进数据库。这个字段名义上是缓存，实际上成了
不可无损重算的正文来源；每次展示规则变化都要改抓取代码并回填历史 Markdown。

数据模型还没有记录一条微博是否真正出现在 tombkeeper.io 时间线。时间线帖、内嵌转发原文以及
为「微博正文」引用按需抓到的帖子最终都落成同一种 `Post`；`GetLatestPosts` 只按发布时间取数，
因此非时间线帖子可能进入 RSS。现有 `PostExists` 短路还会阻止一条先按引用入库的帖子在以后真正
出现在时间线时升级身份。

## 目标

- 抓取期只解析、补全并保存可重放的结构化事实及资源获取结果，不生成或保存 Markdown。
- RSS 与单篇归档在读取事实时统一渲染 Markdown；展示规则只有一个实现落点。
- Markdown renderer 是纯函数：完整事实与显式选项相同，输出必然相同；函数内不查询数据库、
  不联网、不读取全局配置。
- 每条微博显式记录 `in_timeline`，RSS 只读取时间线帖；非时间线帖子仍可作为引用和归档内容。
- 一条帖子一旦在任意列表页被确认位于时间线，后续详情抓取不得把它降回非时间线。
- 保持当前对纯文本、图片、视频、转发、「查看图片」、微博正文短链和页脚的用户可见语义。
- 允许破坏性重建 tombkeeper 微博数据，不为旧 `text_markdown` 设计兼容层或反向解析器。

## 验收

- 持久化模型不再含 `text_markdown`，也不保存可由事实稳定推导的 `title` / `video_url`。
- 抓取模块没有 Markdown 拼装、引用块排版、标题截断、摘要提取或时间文案格式化。
- 读取同一组事实时，RSS 与归档复用同一个 Markdown 渲染模块。
- 已成功解析过的「查看图片」H5 URL 复用数据库事实，后续重复摄取不再请求该 H5 页面；暂时性
  请求失败不写完成标记，允许下次重试。
- 内嵌转发原文和按需抓取的微博正文默认 `in_timeline=false`，不进入 RSS。
- 非时间线帖后来出现在列表页后被提升为 `in_timeline=true`，且该值不会被后续写入降级。
- 回归测试覆盖「较新的非时间线引用帖不会挤进 RSS」。
- 现有 tombkeeper Atom golden 的用户可见内容保持一致；有意差异必须在 plan 中列明。

## 不做什么

- 不改 tombkeeper.io 的抓取范围、小时 cron、历史回填接口或 Redis RSS 缓存架构。
- 不引入通用微博领域框架，不复用休眠的 `pkg/routers/weibo`，不顺带改 tkblog。
- 不做多层递归引用展开；继续只内联一层。
- 不为避免一次重抓历史而保留双写、双读、事实版本或 `text_markdown` 兼容路径。
- 不在本 issue 中引入 LLM 标题。
