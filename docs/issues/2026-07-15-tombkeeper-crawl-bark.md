---
title: "tombkeeper 抓取失败缺少 Bark 聚合告警"
kind: bug
status: closed
priority: high
areas: [tombkeeper, cron, notify]
plan: docs/plans/2026-07-15-tombkeeper-crawl-bark.md
related: [cmd/server/cron.go, pkg/routers/tombkeeper/crawl.go, pkg/routers/tombkeeper/history.go]
updated: "2026-07-15"
---

## 问题

每小时 `tombkeeper_crawl` 没有注入 Bark notifier，顶层失败只写日志；单条微博入库、H5 解析、引用抓取
或图片归档失败又会被 importer 记录后继续，最终只体现在 `EntriesFailed` 或日志中。2026-07-14 至
2026-07-15 的 H5 图片索引 UPSERT 故障因此连续发生但没有 Bark。手工运行同名 cron job 复用相同闭包，
也没有通知。history 回填虽会通知顶层 error/panic，但单条失败同样只统计、不通知。

## 目标

- live crawl 的顶层 error 或 panic 必须发送 Bark。
- live/history 的单条可恢复失败继续处理其他微博，但每次 run 结束后最多发送一条聚合 Bark。
- 聚合通知包含 run/job 标识、失败数量和可定位的代表性错误，不逐条轰炸。
- 无错误的正常抓取不发送 Bark；Bark 发送失败继续由通知模块记录日志，不影响抓取结果。

## 验收

- 回归测试证明 live crawl 遇到单条 UPSERT 失败时继续执行，并只发送一条聚合 Bark。
- 回归测试证明 live crawl 顶层 error 与 panic 各只发送一条 Bark，正常 run 不通知。
- 回归测试证明 history 单条失败结束时只发送一条聚合 Bark；既有顶层 error/panic 通知不重复。
- 手工 `/api/v1/job/run/tombkeeper_crawl` 与小时任务复用同一通知语义。
- tombkeeper 目标测试与 `just lint` 通过。

## 不做什么

- 不改变单条失败后继续抓取的容错语义，不把任一单条失败升级为整次中止。
- 不引入跨 run 的持久化告警、去重窗口或全局通知框架。
- 不修改 Bark 服务配置、生产凭据、抓取频率或 RSS 内容。
