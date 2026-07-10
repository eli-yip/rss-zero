# SPEC: tombkeeper 历史回填

日期：2026-07-06

## 背景与目标

现有 tombkeeper 抓取只有每小时 cron 抓最新 2 页（`?page=N`），无法补历史。
tombkeeper.io 支持日期窗口分页：

```
https://tombkeeper.io/?startDate=2026-06-01&endDate=2026-06-26&page=1
```

目标：新增一个「指定起止日期、向后（新→旧）连续回填」的抓取能力，把某个历史窗口的
微博一次性补进库。live cron 继续负责最新内容，本功能只做向后回填。

## 实测确认（真实请求 tombkeeper.io）

窗口 `2026-06-01..2026-06-26`：

- 日期参数生效，按页倒序：page1 最新（贴近 endDate），page 增大 → 更旧。
- 页间无重叠：page1/page2 各 5 条详情链接，交集为空 → 翻页连续无缝。
- 越界页返回空：page99 → 0 条 `/weibo/{id}` 详情链接 → **「空页 = 窗口抓完」是可靠停止信号**。
- **转发原文带窗口外旧日期**：page1 只有 5 条时间线帖，flight 里却出现 `2026-05-28`
  等 created_at（早于 startDate）——这是内嵌的转发原文。**时间边界判断必须只看时间线帖，
  不能被转发原文的旧日期污染。**

## 需求

1. 抓取函数指定开始/结束日期（`YYYY-MM-DD`，Asia/Shanghai）。
2. retweet 例外：转发原文可能是旧的，不得影响「最早博文时间」/边界判断。
   —— 现有 `timelineIDs(html)` 只取时间线帖（不含转发原文），边界只走它即可天然规避。
3. 参考知识星球/知乎的历史抓取套路（窗口/游标翻页 + 到边界停止 + 幂等去重）。
4. 触发方式、失败处理（本次决策）：
   - 独立 admin 接口触发（非通用 job 框架）。
   - 不需要断点续传；但抓取失败要报错并 Bark 通知。
   - 日期按 Asia/Shanghai 解释。
   - `maxHistoryPages = 8000`（注释标注为临时上限）。
   - 返回一个 `job_id`（xid，非持久化）：进日志字段、进返回体、失败 Bark 内容都带上，用于 grep 追踪单次抓取；不落库、不查状态。
   - 进程内禁止两个 history 并发：第二次触发直接拒绝（409），不排队。

## 非目标

- 断点续传 / 进度查询接口。
- 与 live cron 的时间协调（靠 `PostExists` 幂等去重，重叠无害）。
- 回填后预热 RSS 缓存（补的是更旧的帖，不进 top-30 feed）。

## 并发

- 回填**不持 `crawlMu`**，与每小时 live 抓取**并发**跑，互不阻塞（否则一个数小时的回填会把 live 卡住几小时——gocron 单例 `LimitModeReschedule` 会跳过这期间的整点，最新帖延迟入库）。并发安全的依据：所有抓取写库都是幂等 upsert + `PostExists` 预检，各自独立 `Requester`/`Renderer`。
- `crawlMu` 仅防 live 自重叠；`historyRunning`（atomic CAS）仅防两个回填并发。

## 设计要点

- `Requester` 加 `GetPageRange(startDate, endDate string, page int)`，拼日期窗口 URL；
  live 用的 `GetPage` 不动。
- 把现有 live `Crawl` 单页入库逻辑抽成 `ingestPage(...) (seen, saved int)`，
  live 与 history 共用。`seen` 只计时间线帖 → 停止条件与 retweet 例外集中一处。
- `runHistory(db, file, startDate, endDate, logger)`：**不持 `crawlMu`**（见「并发」节，与 live 并发），
  `for page:=1; page<=maxHistoryPages; page++` 翻页，`seen==0` 即停。`StartHistory` 外层 CAS 守卫 + 生成 job_id + 后台 goroutine。
- admin 接口 `POST /api/v1/tombkeeper/history {start_date,end_date}`：校验日期格式 + `start<=end`（trust boundary），
  调 `StartHistory`（占用中 409，否则 202 带 job_id）；失败记日志 + Bark。
