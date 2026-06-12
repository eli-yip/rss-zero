# PLAN：项二 — 删除 zsxq 爬取回溯死代码

> 对应 SPEC：[2026-06-12-01-zsxq-maintainability-refactor.md](../specs/2026-06-12-01-zsxq-maintainability-refactor.md) 重构项二
> 分支：`feat-zsxq-drop-backtrack`
> 风险：最低（纯删除死代码，正向爬取路径不动）

## 目标

删除 `CrawlGroup` 中永不执行的回溯分支及其专属参数、DB 方法，使分页爬取只剩一条正向路径。

## 前置事实（已核实）

- `CrawlGroup` 全项目唯一调用点：`pkg/routers/zsxq/cron/crawl.go:277`，恒传 `backtrack=false, earliestTopicTimeInDB=time.Time{}`。
- 回溯循环 `group.go:92–144` 永不执行。
- `GetEarliestTopicTime`（`db/topic.go:31` 接口声明 + `:68` 实现）无任何调用者，无 mock 实现。
- 作者已确认无运维手动触发回溯的历史用法。
- `group.go` 现有 import 在正向循环中均仍被使用，删除回溯后无 unused import。

## 步骤

### 1. 建分支
```
git checkout -b feat-zsxq-drop-backtrack
```

### 2. 改 `pkg/routers/zsxq/crawl/group.go`
- 删除回溯循环及其引导语句：`group.go:92–144`（`if !backtrack { return nil }` 起，到回溯 `for` 结束）。正向循环结束后直接 `return nil`。
- 从 `CrawlGroup` 签名删除 `backtrack bool` 与 `earliestTopicTimeInDB time.Time` 两个参数（L25）。
- 函数体内若有 `finished = false` / `createTime = earliestTopicTimeInDB` 的回溯重置语句，一并随循环删除。

### 3. 改唯一调用点 `pkg/routers/zsxq/cron/crawl.go:277`
- 调用从 `crawl.CrawlGroup(groupID, requestService, parseService, latestTopicTimeInDB, false, false, time.Time{}, logger)` 改为去掉末两个布尔/时间实参：`crawl.CrawlGroup(groupID, requestService, parseService, latestTopicTimeInDB, false, logger)`。
- 检查该文件是否因此产生 unused 的 `time` import（`latestTopicTimeInDB` 仍用 `time.Time`，预计仍需保留——以 `go build` 为准）。

### 4. 删除 `GetEarliestTopicTime`
- `pkg/routers/zsxq/db/topic.go`：删除接口声明（L30–31 注释 + 方法）与实现（L68–77）。

### 5. 校验
```
go build ./...
go vet ./pkg/routers/zsxq/...
```
- 针对性测试（若有）：`go test ./pkg/routers/zsxq/crawl/... ./pkg/routers/zsxq/db/...`
- 不跑全量测试套件（遵循 AGENTS.md）。

## 验收

- `go build ./...` 通过，无 unused import/参数。
- `CrawlGroup` 仅余正向循环，签名不含 `backtrack`/`earliestTopicTimeInDB`。
- `GetEarliestTopicTime` 从接口与实现中移除。
- 正向爬取行为不变（调用点行为等价：原本 `backtrack=false` 即只走正向）。

## 提交与合并

- 小步提交，Conventional Commit（英文），如：
  `refactor(zsxq): drop dead backtrack branch from CrawlGroup`
- 完成后请作者 review，批准后 squash merge 进 `master` 并删分支。
- 更新 `docs/PROGRESS.md`。
