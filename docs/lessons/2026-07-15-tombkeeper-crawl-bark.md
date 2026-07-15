---
title: "tombkeeper 抓取错误必须在 run 边界聚合"
plan: docs/plans/2026-07-15-tombkeeper-crawl-bark.md
status: done
areas: [tombkeeper, cron, notify]
updated: "2026-07-15"
---

# tombkeeper 抓取错误必须在 run 边界聚合

H5 图片索引故障持续触发 UPSERT error 却没有 Bark，不是单一漏调：importer 有意把单条失败降级为
`EntriesFailed` 后继续，而 live cron 闭包既没有 notifier，也只会记录向上返回的 fatal error。只在最外层
给 `error != nil` 加 Bark 仍会漏掉这类最常见的可恢复失败。

本次把错误分成两层：页面请求、解析、批量读库、RSS/Redis warm 等 fatal error 继续中止 run；引用抓取、
H5 解析、单条 UPSERT、图片归档与 missing timeline entry 进入 `FailureSummary`，继续处理其他微博。摘要
只保留 3 条代表性错误，但总数完整；missing entry 的解析模型原本只保留数量，因此诚实报告阶段与数量，
不为告警反向扩张解析结构。

通知决策放在 live `CrawlFunc` 和 history 后台 goroutine 的 run 边界：panic、fatal、成功但有 partial failure
三者互斥，每次 run 最多一条 Bark；正常 run 不通知。fatal 通知合并此前累计的 partial 摘要，所以既不丢
线索，也不会重复推送。手工 run-now 复用 scheduler 中同一个 live 闭包，天然得到相同语义。

plan review 还发现 Bark 自身的 transport error 会在 `resp == nil` 时解引用并二次 panic。先用可控 HTTP
transport 写红灯，再让 `Notify` 在读取 status 前返回网络错误，并在成功或非 200 时关闭 body。否则最需要
告警的 panic defer 反而可能被通知代码击穿。

实现评审进一步补齐了两个边界：Go HTTP client 也允许同时返回 response 与 error，因此 error 分支仍须关闭
非 nil body；限制代表性错误条数还不足以保证 Bark 有界，还要限制单项文本与整条通知的 rune 数。最终以真实
`TimelineImporter` 驱动两个连续 UPSERT 失败，证明 importer 继续处理，run 边界只发一条汇总通知。Standards
与 Spec 复审均 PASS。

红灯分别证明：Bark notifier 无安全 transport seam；live/history 没有聚合通知入口；UPSERT、H5、图片
归档、引用抓取与 missing entry 不进入摘要。绿灯后目标命令
`go test ./pkg/routers/tombkeeper/... ./internal/notify/... ./cmd/server/... -count=1` 通过；history 异步测试以
完成 channel 同步，不依赖 sleep。`go test -race ./pkg/routers/tombkeeper/... ./internal/notify/... -count=1`
与 `just lint` 同样通过。
