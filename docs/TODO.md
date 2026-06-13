# TODO

记录 PLAN 之外、值得后续跟进的大小需求与技术债。每条注明来源与背景，便于冷启动时直接动手。

## Lint 现代化后待清理项

2026-06-13 将 `just lint` 对齐 maestro-engine，新增 `autocorrect`、`dprint check`、`go mod tidy -diff`、`go fix --diff` 四道检查（`golangci-lint` 本身 0 issue）。本轮只接入工具、不修存量问题，待清理如下：

### 1. autocorrect（CJK/Latin 间距，233 处）

`autocorrect --lint .` 报 233 处中英文/数字间距问题。大头在渲染测试的 golden HTML，**改前需确认是否会破坏快照测试**：

- `pkg/render/test/test_article_1.html`（14）、`pkg/render/test/test_inline_article_1.html`（13）
- `pkg/routers/zhihu/example/content_html/answer_with_pic_and_card.html`（4）、`pin_with_title.html`（1）

以下为普通源码/测试，可安全 `autocorrect --fix`：

- `pkg/routers/macked/filter_test.go`（3）
- `pkg/routers/zsxq/export/export_test.go`（4）
- `pkg/routers/zsxq/time/time.go`（2）、`time_test.go`（2）
- `pkg/routers/xiaobot/render/full_text.go`（1）
- `pkg/routers/zhihu/export/single.go`（1）、`pkg/routers/zhihu/render/time.go`（1）
- `docs/plans/2026-06-12-03-zsxq-object-uri.md`（1）

### 2. dprint（Markdown 格式化，12 个文件）

`dprint check` 报 12 个 Markdown 未格式化，`dprint fmt` 可一键修复（注意会改动既有 docs 排版）：

- `AGENTS.md`、`docs/PROGRESS.md`
- `docs/specs/`：`2026-06-11-01`、`2026-06-11-02`、`2026-06-12-01`、`2026-06-12-02`
- `docs/plans/`：`2026-06-11-02`、`2026-06-12-01`、`2026-06-12-02`、`2026-06-12-03`、`2026-06-12-04`
- `docs/lessons/2026-06-11-02-unified-cookie-interface.md`

### 3. go.sum 未 tidy

`go mod tidy -diff` 显示 `go.sum` 缺少若干哈希条目（`go.uber.org/goleak`、`golang.org/x/exp`、`gopkg.in/check.v1` 等）。运行 `go mod tidy` 补齐。

### 4. go fix 现代化建议（17 个文件）

`go fix --diff ./...` 建议的现代化改写，涉及 `strings.Builder` 替代 `+=` 字符串拼接、`new(expr)`（Go 1.26）替代 `getStringPtr` 泛型辅助并加 `//go:fix inline` 等。逐文件审阅后用 `go fix ./...` 应用：

`cmd/cli/view.go`、`internal/controller/archive/zsxq_archive.go`、`internal/controller/zhihu/export.go`、`internal/controller/zsxq/export.go`、`internal/md/basic.go`、`internal/redis/redis.go`、`pkg/render/html.go`、`pkg/render/html_test.go`、`pkg/routers/xiaobot/refmt/refmt.go`、`pkg/routers/zhihu/cron/export.go`、`pkg/routers/zhihu/refmt/answer.go`、`pkg/routers/zhihu/refmt/article.go`、`pkg/routers/zhihu/refmt/pin.go`、`pkg/routers/zhihu/render/html_convert.go`、`pkg/routers/zsxq/random/random.go`、`pkg/routers/zsxq/refmt/refmt.go`、`pkg/routers/zsxq/render/html.go`
