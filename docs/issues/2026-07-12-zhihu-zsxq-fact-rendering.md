---
title: "知乎与知识星球：只存事实，读取时渲染 Markdown"
kind: refactor
status: open
priority: high
areas: [zhihu, zsxq, rss, controller, migrate, db, render, file]
plan: docs/plans/2026-07-12-zhihu-zsxq-fact-rendering.md
related: [pkg/routers/zhihu, pkg/routers/zsxq, internal/rss, internal/controller/archive]
updated: "2026-07-12"
---

## 问题

知乎与知识星球是同一个病：抓取路径把「解析事实」和「排版展示」揉在一起，把已排版的 Markdown 冻结进
一个名为缓存、实为唯一正文来源的 `text` 列。

- 知乎 `ParseAnswer`/`ParseArticle`/`parseAndSavePin`（`pkg/routers/zhihu/parse/*`）在保存前完成
  HTML→Markdown、图片换 OSS 链、付费提示、pin 块与引用排版、格式化，写进 `zhihu_{answer,article,pin}.text`。
- 星球 `ParseTopic`（`pkg/routers/zsxq/parse/topic.go`）在保存前调 `render.Text`，拼作者头/图片/文件/
  语音块/外部文章/9 道格式化，写进 `zsxq_topic.text`。
- 两源 refmt（`pkg/routers/{zhihu,zsxq}/refmt`）从 `raw` 重做写入期渲染，副本已与 parse 路径漂移。
- `internal/rss/fetch_{zhihu,zsxq}.go`、archive、export、random 直接消费 `text`，使数据库 Markdown 成了
  正文权威版本；任何展示规则调整都要改抓取代码 + 全库回填（refmt 无断点续传、只按 id 校验）。
- 图片/附件/语音解析会**先**写 Object，内容根行**后**写；中途失败留下「有 Object、没内容行」的可见半态。

## 目标

- 抓取期只解析并保存**结构化事实**（原始载荷 `raw` + 作者/问题/对象/外部文章等侧表），不**保存** Markdown。
- RSS、归档、导出、random 读取事实时统一由**一个纯函数** `RenderMarkdown` 渲染；展示规则只有一个落点。
- 纯 renderer：相同的自包含快照 + 显式 `serverBaseURL` 必产逐字节相同输出；不查 DB、不联网、不读配置、
  不取当前时间。
- 破坏性**原地删 `text` 列，不 truncate、不重抓、不留任何回退列**——线上实测 `raw` 完整、可原地重放
  （见 plan 决策 1/7）。
- 每条内容产生的 Object/Author/Question/Article + 内容根行在**同一事务**内可见（关掉现有半态 bug）。
- 保持两源全部现有用户可见语义（skip/error/save 行为矩阵、付费提示、图片、语音转写、外部文章、
  question 标题、精华、摘要、原文链接）。

## 验收

- 五张内容表不再含 `text` 列；抓取模块不**保存** Markdown（允许 transient 纯渲染喂派生事实，不持久化）。
- RSS 与归档/导出/random 读取同一组事实时复用同一个纯 `RenderMarkdown`。
- 破坏性迁移原地 `DROP COLUMN text`（五表统一、无新增列），保留每一行 `raw`，无数据丢失、无重抓作业；
  付费在读取期从 `raw` 推导渲染、**不存列**（SSOT，见 plan 决策 5）。
- Object/内容根行同事务；初次抓取失败不留半态，更新失败保留旧行。
- 现有 `internal/rss/testdata/{zhihu_*.atom,zsxq.atom}` 用户可见内容一致；新增**经真实 `RenderMarkdown`**
  的端到端渲染测试（补当前 golden 用平凡纯文本、不经渲染的盲区）；行为契约矩阵逐行有测试。
- 实施内部分两阶段：先 ZSXQ pilot 证明模式，再 Zhihu delta。

## 不做什么

- **不 truncate 内容表、不做全量重抓**（那会丢掉已删除/已失权限/抓不回的历史；实测证明也无必要）。
- **不保留 `text`/`legacy_text` 回退列**、不双写双读、不做反向解析器兼容层。
- 不引入跨源内容框架、来源无关 AST、facts version 或内容完成状态。
- 不为编译保留过时的一次性历史迁移；按需**直接删除**（作者放行）。
- 不动 `pkg/render` 里被 xiaobot/github/endoflife 等共用的 HTML→MD 规则（只动 zhihu 专属的
  `figure`/`data-original`）。
- 不渲染 zvideo（旁路下载，不产 text、不进 RSS）；不改抓取范围、cron、加密服务对接或 Redis 缓存架构
  （仅换 source cache key 隔离）。
- 不在本次引入新的 LLM 能力；AI 标题保持派生事实存 `title`。
- 不引入对象存储 staging/不可变 key/orphan 回收（列后续项）。
