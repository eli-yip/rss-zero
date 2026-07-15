---
title: "为 tombkeeper live/history 抓取增加单次聚合 Bark"
issue: docs/issues/2026-07-15-tombkeeper-crawl-bark.md
status: done
areas: [tombkeeper, cron, notify]
updated: "2026-07-15"
---

# PLAN: 为 tombkeeper live/history 抓取增加单次聚合 Bark

## 目标

解决 [issue](../issues/2026-07-15-tombkeeper-crawl-bark.md)：让每小时与手工触发的 live crawl、history
回填在失败时都有一致、可定位且不轰炸的 Bark；保留“单条失败继续处理”的既有容错语义。

## 关键决策

### 1. importer 返回有界失败摘要，不返回首个单条 error

新增 run 内存态失败摘要，记录总失败数和至多 3 条代表性错误。`TimelineImporter.Import` 在现有
`ImportStats` 中返回摘要，覆盖以下继续执行的失败：时间线载荷缺失条目、引用微博抓取、H5 图片索引解析、
博文 UPSERT、图片归档。摘要中的错误带阶段，并在现有数据可得时带 post/image 标识；`MissingEntries`
在解析结果中只有数量，故只报告 `timeline payload missing entries: N`，本次不为告警反向扩张解析模型。
超过示例上限只增加总数，避免一次异常页面造成无界内存或超长 Bark。

`EntriesFailed` 继续只表示失败的时间线条目，不改变统计含义；单条失败仍然记录原日志并继续处理。顶层
页面请求、页面解析、批量读库、RSS 构建或 Redis warm 等错误仍通过 `error` 中止本次 run。

### 2. 每个 run 只有一个通知决策点

live crawl 的公开 `CrawlFunc` 注入 notifier，并在闭包边界统一处理结果与 panic：

- panic：发送一条 `Tombkeeper crawl panicked`；
- 顶层 error：发送一条 `Tombkeeper crawl failed`，附 run id、fatal error 和此前累计的单条失败摘要；
- 无顶层 error、但摘要总数大于 0：发送一条 `Tombkeeper crawl completed with errors`；
- 无错误：不通知。

history 的后台 goroutine 使用同一套摘要格式与“一个 run 最多一条”规则，保留现有 error/panic 标题；成功
但有单条失败时补一条 completed-with-errors。panic 或 fatal error 已经代表本次结果时，不再另发摘要通知。

Bark 发送继续走 `notify.NoticeWithLogger`；同时修复 `BarkNotifier.Notify` 的 transport error 路径：
`http.Get` 返回 error 时必须在读取 `resp.StatusCode` 前直接返回，并关闭非 nil response body。发送失败只记录
日志，不重试、不改变 crawl 结果，尤其不能在 crawl 的 panic defer 中因二次 panic 击穿进程。

### 3. 小型 runner seam 让闭包行为可测

把 live/history 的“执行抓取”与“解释结果并通知”分开：生产 runner 负责真实网络、DB 与缓存；公开入口仍是
`CrawlFunc` / `StartHistory`，内部构造函数接收 runner function，测试用 fake runner 驱动成功、部分失败、
fatal error 与 panic。history 测试通过 runner 完成 channel 等待后台 goroutine，不用 sleep。测试只通过
闭包入口和 fake notifier 观察通知次数/内容，不断言私有 helper 调用，也不访问真实 Bark 网络。

手工 `/api/v1/job/run/tombkeeper_crawl` 继续由 scheduler 运行同一个 `CrawlFunc`，无需 controller 特判。

## 代码落点

- `pkg/routers/tombkeeper/import.go`：失败摘要类型与 importer 各可恢复失败的采集。
- `pkg/routers/tombkeeper/crawl.go`：live run report、notifier 注入、panic/fatal/partial 的单点聚合通知。
- `pkg/routers/tombkeeper/history.go`：跨页合并摘要，并在后台 run 边界复用同一通知格式。
- `cmd/server/cron.go`：向 tombkeeper 静态 cron 注入现有 Bark notifier。
- `internal/notify/bark.go`：安全处理 HTTP transport error 并关闭 response body。
- `internal/notify/bark_test.go`：Bark transport error 返回 error、不 panic 的回归测试。
- `pkg/routers/tombkeeper/*_test.go`：fake notifier/runner 的行为回归与 importer 失败摘要测试。

## 实施步骤（对应提交）

1. **红灯：live 通知闭包**：先写 `CrawlFunc` 的正常、partial、fatal、panic 表驱动测试；旧签名/行为无法满足
   notifier 断言，记录红灯后实现 notifier 注入与单次通知决策。
2. **红灯：失败摘要完整性**：逐个增加 importer 的 UPSERT、H5 与图片归档代表性用例，先证明旧实现只有
   日志/`EntriesFailed`，再实现有界摘要；引用抓取与 missing entry 用同一摘要 API 覆盖。
3. **红灯：history 聚合**：扩展既有 history fake，证明单条失败最终触发一次 Bark、fatal/panic 不重复；
   实现跨页摘要合并和 completed-with-errors 通知。
4. **红灯：通知 transport 安全**：用关闭的 `httptest.Server` 证明 Bark transport error 当前会 panic；
   修复 nil response 处理与 body 关闭，使其只返回 error。
5. **收尾**：创建 lesson，更新 issue/plan/PROGRESS；运行目标测试与 `just lint`，执行 `/code-review`
   Standards + Spec 双审并处理 findings，squash 合并 master 后发版部署。

## 测试

- `CrawlFunc`：正常 0 通知；partial/fatal/panic 各 1 通知；通知含 run id、失败总数及代表性错误；fatal 与
  已累计 partial 不重复发送。
- importer：UPSERT、H5、引用抓取、图片归档与 missing entry 都进入摘要；仍继续处理后续条目；示例最多
  3 条但总数准确；missing entry 只要求阶段与数量，其他示例断言可得的 post/image 标识。
- history：跨页合并 partial 后只发 1 条；现有 fetch/pagination/import fatal error 仍发 1 条；panic 只发
  1 条；正常结束 0 通知。
- Bark：HTTP transport error 返回 error 且不 panic；成功/非 200 response body 均被关闭。
- `go test ./pkg/routers/tombkeeper/... ./internal/notify/... ./cmd/server/...`；不运行全量测试。
- `just lint`。

## 待更新文档

- [x] `docs/lessons/2026-07-15-tombkeeper-crawl-bark.md`：记录错误分级、聚合边界与红绿证据。
- [x] `docs/issues/2026-07-15-tombkeeper-crawl-bark.md`：实现评审通过后改为 `closed`。
- [x] `docs/plans/2026-07-15-tombkeeper-crawl-bark.md`：开工时改 `in-progress`，squash 后改 `done`。
- [x] `docs/PROGRESS.md`：记录实现、验证、发版和生产部署结果。
- [x] `docs/ARCHITECTURE.md`：补充 tombkeeper 静态 cron 的聚合告警语义。
- [x] `docs/OPS.md`：无需修改；部署步骤与人工门禁不变。
- [x] `docs/TODO.md`：未发现新的计划外问题；Bark transport 安全修复已由 plan review 纳入当前范围。

## 后续项

跨 run 的告警去重、冷却窗口与统一 crawl observability 不在本次范围；若每小时重复告警造成噪声，再单开
issue 设计，不在本次引入持久状态。
