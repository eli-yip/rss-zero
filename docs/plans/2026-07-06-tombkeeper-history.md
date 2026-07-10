# PLAN: tombkeeper 历史回填

日期：2026-07-06 · SPEC: [issues/2026-07-06-tombkeeper-history.md](../issues/2026-07-06-tombkeeper-history.md)

## 步骤

1. `pkg/routers/tombkeeper/request.go`：`Requester` 加 `GetPageRange(startDate, endDate string, page int) ([]byte, error)`，
   实现拼 `?startDate=..&endDate=..&page=N`（日期已在 handler 校验，无需再转义）。

2. `pkg/routers/tombkeeper/crawl.go`：抽 `ingestPage(html []byte, db DB, renderer *Renderer, logger) (seen, saved int)`
   —— 搬现有 live 循环体（`ExtractPosts` → `timelineIDs` 两源对账 → `PostExists` 去重 → `Render` → `SavePost`）。
   `seen` 只数时间线帖。live `Crawl` 改为逐页调用 `ingestPage`（`logger.With(page)`），行为不变。

3. `pkg/routers/tombkeeper/history.go`（新）：
   - `const maxHistoryPages = 8000`（ponytail 注释：临时上限，正常靠空页停止）。
   - `crawlHistoryPages(req, db, renderer, startDate, endDate, logger)`：翻页 `ingestPage`，`seen==0` 停、到 cap warn。
   - `runHistory(...)`：**不持 `crawlMu`**（与 live 并发跑，幂等 upsert 保证安全）；新建 `Requester`/`Renderer`；调 `crawlHistoryPages`。
   - `StartHistory(db, f, notifier, startDate, endDate, logger) (jobID string, err error)`：`historyRunning atomic.Bool`
     CAS 守卫（占用中返回 `ErrHistoryRunning`，同步拒绝、不排队）；生成 `xid` job_id；后台 goroutine 跑 `runHistory`，
     日志/失败 Bark 都带 job_id。
   - 自检：翻到空页停 + 幂等；转发原文旧日期不入库不影响停止；`StartHistory` 在占用中拒绝第二次。

4. `internal/controller/tombkeeper/controller.go`：`Controller` 加 `file file.File`、`notifier notify.Notifier` 字段，
   `NewController` 增参。

5. `internal/controller/tombkeeper/history.go`（新）：`History` handler —— bind + 校验 `YYYY-MM-DD`（`config.C.BJT`），
   调 `tk.StartHistory`；占用中（`ErrHistoryRunning`）返回 409，否则 202 带 `{job_id}`。

6. `cmd/server/echo.go`：`NewController` 补 `fileService, notifier`；新增 admin 组
   `/api/v1/tombkeeper` + `POST /history`，并入 `groupNeedAuth`（自动套 `AllowAdmin`）。

7. `just lint` + 改动包测试。

## 验证

- `ingestPage` 抽取后 live crawl 单测/行为不回退。
- `CrawlHistory` 自检：空页停止 + retweet 旧日期不影响。
- 手动：admin POST 一个小窗口，观察日志抓取条数；构造失败（如断网）确认 Bark。
