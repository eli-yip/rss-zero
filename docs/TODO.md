# TODO

记录 PLAN 之外、值得后续跟进的大小需求与技术债。每条注明来源与背景，便于冷启动时直接动手。

## Lint 现代化后待清理项

2026-06-13 将 `just lint` 对齐 maestro-engine，新增 `autocorrect`、`dprint check`、`go mod tidy -diff`、`go fix --diff` 四道检查，并新增 `just fix-lint` 一键自动修复。**下列四项已于 2026-06-13 全部清理，`just lint` 现已 0 issue。** 保留在此作为处理记录。

> 修正：当初记的「`golangci-lint` 本身 0 issue」并不准确——见下方 [issue B](#b-golangci-lint-并非-0-issue已修正)。

清理过程中另发现两点需留意：

- **issue A — autocorrect 对 Go 运行时字符串不安全**：见第 1 项。`xiaobot/render/full_text.go` 等处的日期渲染格式、`zhihu/export/single.go` 的导出文件名都是运行时输出而非散文，autocorrect 会插入 CJK 空格（如 `2006 年 1 月 2 日`）改变实际行为，并把 `export_test.go` 期望误改成源码无法拼出的 `知识星球合集 -28855…`。已用 `// autocorrect-disable`/`-enable` 就地豁免、保持行为不变。**后续若再跑 `just fix-lint`/`autocorrect --fix`，务必保留这些 guard，不要删除。** 是否在渲染输出里正式采用 CJK 加空格，属产品决定，留待后续单独讨论。
- **issue B — golangci-lint 并非 0 issue（已修正）**：`go fix` 把 `getStringPtr`/`GetPtr` 的调用点全部内联到 `new(expr)` 后，这两个辅助函数变为死代码，被 `unused` 检出并删除；同时发现一个与本轮无关的预存死类型 `internal/controller/zhihu/author.go` 的 `ErrResponse`（统一响应封装 d9d2d20 重构后遗留），一并移除。`golangci-lint` 现 0 issue。

### 1. autocorrect（CJK/Latin 间距）— ✅ 已处理（2026-06-13）

处理方式：

- **快照/捕获类 fixture 加入 `.autocorrectignore`**：`pkg/render/test/`（golden HTML）与 `**/example/`（捕获的 API 响应 JSON/HTML，须与上游字节一致，autocorrect 会破坏真实昵称/内容并误改 `合集-数字` 之类文件名）。
- **源码里被误判的字符串/数据用 `// autocorrect-disable`/`-enable` 就地豁免，保持运行时行为不变**：日期渲染格式（`xiaobot/render/full_text.go`、`zhihu/render/time.go`、`zsxq/time/time.go`）、导出文件名（`zhihu/export/single.go`），以及对应的测试期望（`zsxq/export/export_test.go`、`zsxq/time/time_test.go`）和真实站点标题 fixture（`macked/filter_test.go`）。注：autocorrect 曾把 `export_test.go` 期望误改为 `知识星球合集 -28855…`，而源码 `export.go:130` 的 `"知识星球合集"` 在运行时拼接，二者无法一致——证实这些字符串不能盲改。
- **真正的散文（docs 计划文档）已 `autocorrect --fix`**。

`just lint` 的 autocorrect 一步现已 0 issue。

### 2. dprint（Markdown 格式化）— ✅ 已处理（2026-06-13）

`dprint fmt` 已格式化 14 个 Markdown（强调风格、间距等），`dprint check` 现已通过。

### 3. go.sum 未 tidy — ✅ 已处理（2026-06-13）

`go mod tidy` 已补齐缺失的 `go.sum` 哈希条目，`go mod tidy -diff` 现已干净。

### 4. go fix 现代化建议（17 个文件）— ✅ 已处理（2026-06-13）

`go fix ./...` 已应用：`interface{}`→`any`、`for i := 0; i < n; i++`→`for range n`、`strings.Split`→`strings.SplitSeq`（range-over-func）、`HasPrefix`+`TrimPrefix`→`CutPrefix`、去除多余的 `x := x` loop-var 拷贝、用 `new(expr)`（Go 1.26）内联 `&v` 指针辅助。内联后空置的 `getStringPtr`/`GetPtr` 已删除（见 [issue B](#b-golangci-lint-并非-0-issue已修正)）。

## 既有测试失败（与 lint 无关，待查）

`go test ./pkg/render/` 的 `TestHtmlRender` 在本地一直失败：实际输出比 golden 多包一层 `<div class="content">…</div>`。在 lint 现代化之前的 commit 上即可复现，属预存问题，非本轮改动引入。需确认是 golden 陈旧还是渲染逻辑回归。

`go test ./internal/controller/archive/` 的 `TestExtractAnswerID` 也是**预存**失败（在 master 上即可复现，非 archive-format 改动引入）：`ExtractAnswerID` 早已改为返回 `*zhihuAnswer`，但该用例仍按旧签名拿它和字符串 `answerID` 比较，必然 not-equal。修复方向：把用例改成 `result.answerID`（或 `strconv.Itoa(result.answerID)`）再断言。

## 统一响应格式后续（unified-response-format）

- **`/statistics` 页面在统计数据为空时白屏崩溃**（预存 bug，与统一响应格式无关）。webapp 的 `<ActivityCalendar>`（react-activity-calendar）在收到空数据时抛异常，且无 error boundary，导致整页不渲染。复现：当 canglimo 在"过去一年"窗口内没有回答时（如本地 DB 数据陈旧），`GET /api/v1/archive/statistics` 返回空 `data`，前端解包为空 → 组件崩溃。生产环境有近一年数据故正常。修复方向：`StatisticsPage`/`Statistics` 组件加空数据 guard（空时显示占位/提示），或包一层 error boundary。
- **后续 API 设计议题**（首性原理讨论结论，留作后续 SPEC）：读操作 POST→GET、content_type 字符串/整数枚举统一、bookmark 子资源化（`PUT/DELETE /topics/{id}/bookmark`）、`/sub/sub/zhihu` 笔误路径修复。
