# PLAN: 知乎付费内容标识（答案 / 文章）

实现 [SPEC 2026-06-20-01](../specs/2026-06-20-01-zhihu-paid-content-flag.md)。无新增数据库列；
付费判定按 `raw` 现算，命中后把带原文链接的引用块烘焙进 `.Text`。小步提交。

依赖事实（已核实）：

- `parse` 包已 import `render`（[parse/parse.go:16](../../pkg/routers/zhihu/parse/parse.go)），
  故可直接用 `render.GenerateAnswerLink/GenerateArticleLink`，无 import 环。
- `md.Quote` 产出 `>` 引用块、`md.Join` 以空行连接块。
- 旧提示 `AddPaidColumnContentNotice` 仅 3 处引用：
  [parse/answer.go:92](../../pkg/routers/zhihu/parse/answer.go)、
  [refmt/answer.go:89](../../pkg/routers/zhihu/refmt/answer.go)、
  [parse/answer_test.go](../../pkg/routers/zhihu/parse/answer_test.go)。
- 迁移路由在 [cmd/server/echo.go:423 registerMigrate](../../cmd/server/echo.go)，
  控制器在 [controller/migrate/migrate.go](../../internal/controller/migrate/migrate.go)。

## 步骤

### 步骤 1：付费判定 + 提示烘焙的共享 helper（parse 包）

新建 `pkg/routers/zhihu/parse/paid.go`：

- `IsPaidAnswer(answerType string) bool` → `== "paid_column_content"`。
- `IsPaidArticle(articleType string, paidInfo json.RawMessage) bool` → 主信号或 `paid_info`
  非空（`!= ""/"{}"/"null"`），带 SPEC 要求的 NOTE 注释说明那 21 篇边界。
- `AddPaidNotice(text, link string) string`：**纯前置**——`md.Quote("本文为付费内容，请点击 [原文链接]("+link+") 查看全文")` 拼到去左空白的 text 之前；text 为空时仅返回引用块。
  解析/重排传入的都是从 raw 重新渲染的新鲜正文，本身不含提示，故无需 strip 与幂等
  （这两件事是一次性的，放进步骤 5 的迁移里，详见该步）。
- 常量：`answerTypePaidColumnContent`、`articleTypePaidColumnContent`（同值 `paid_column_content`）。
- 删除 [parse/answer.go](../../pkg/routers/zhihu/parse/answer.go) 内旧的
  `AddPaidColumnContentNotice`、`answerTypePaidColumnContent`、`paidColumnContentNotice`。

更新测试 `parse/paid_test.go`：测 `IsPaidAnswer`、`AddPaidNotice`（前置、去空白、空 text、链接注入）；
新增 `IsPaidArticle` 表驱动用例（覆盖 4 种 article_type×paid_info 组合）。

`just lint` + `go test ./pkg/routers/zhihu/parse/...`。提交 `feat(zhihu): paid detection + linked notice helpers`。

### 步骤 2：article api model 暴露判定字段

[api_models/article.go](../../pkg/routers/zhihu/parse/api_models/article.go) 的 `Article`
增加 `ArticleType string \`json:"article_type"\``与`PaidInfo json.RawMessage \`json:"paid_info"\``（import`encoding/json`）。
答案侧`AnswerType` 已存在，无需改。

`just lint`。并入步骤 3 一起提交（无独立行为）。

### 步骤 3：解析期烘焙（answer + article）

- [parse/answer.go:92](../../pkg/routers/zhihu/parse/answer.go)：把
  `text = AddPaidColumnContentNotice(text, answer.AnswerType)` 换成
  ```go
  if IsPaidAnswer(answer.AnswerType) {
      text = AddPaidNotice(text, render.GenerateAnswerLink(answer.Question.ID, answer.ID))
  }
  ```
  （此处 `answer.Question.ID`、`answer.ID` 已在上文解析完成）。
- [parse/article.go](../../pkg/routers/zhihu/parse/article.go) `ParseArticle`：在
  `formattedText` 生成后、`SaveArticle` 前插入
  ```go
  if IsPaidArticle(article.ArticleType, article.PaidInfo) {
      formattedText = AddPaidNotice(formattedText, render.GenerateArticleLink(article.ID))
  }
  ```
  并把 `Text: formattedText` 用这个变量（注意 article 解析里 `WordCount` 不存，无需动）。

`just lint` + `go build ./...`。提交 `feat(zhihu): bake paid notice on answer/article parse`。

### 步骤 4：重排期烘焙（refmt）

- [refmt/answer.go:89](../../pkg/routers/zhihu/refmt/answer.go)：把
  `text = parse.AddPaidColumnContentNotice(text, answer.AnswerType)` 换成
  ```go
  if parse.IsPaidAnswer(answer.AnswerType) {
      text = parse.AddPaidNotice(text, render.GenerateAnswerLink(a.QuestionID, a.ID))
  }
  ```
  （`a` 是 `db.Answer`，有 `QuestionID`/`ID`；需确认 refmt 已 import render，否则加）。
- [refmt/article.go](../../pkg/routers/zhihu/refmt/article.go)：在 `formattedText` 后、
  `SaveArticle` 前加
  ```go
  if parse.IsPaidArticle(article.ArticleType, article.PaidInfo) {
      formattedText = parse.AddPaidNotice(formattedText, render.GenerateArticleLink(a.ID))
  }
  ```
  （refmt/article.go 当前未 import parse，需新增 import parse + render）。

`just lint` + `go build ./...`。提交 `feat(zhihu): bake paid notice on answer/article refmt`。

### 步骤 5：迁移回填历史正文（20260620）

新建 [internal/migrate/20260620.go](../../internal/migrate/20260620.go) `Migrate20260620`，
仿 [20260612.go](../../internal/migrate/20260612.go)：

- 复用 `xid` + `recover` 框架与逐行 UPDATE 写法。
- answer：投影 `id, question_id, raw, text`；从 `raw` 取 `answer_type`；
  `parse.IsPaidAnswer` 命中 → `text2 := applyPaidNotice(text, render.GenerateAnswerLink(questionID, id))`；
  若 `text2 != text` 则 `UPDATE text`。
- article：投影 `id, raw, text`；从 `raw` 取 `article_type` + `paid_info`；
  `parse.IsPaidArticle` 命中 → `applyPaidNotice(text, render.GenerateArticleLink(id))`；变化则 UPDATE。
- `applyPaidNotice`（迁移本地，连同 `legacyPaidNotice`/`paidNoticePrefix` 常量与
  `stripLegacyPaidNotice`）承担一次性的旧提示 strip 与幂等：已含 `> 本文为付费内容` 前缀则
  原样返回，否则 strip 旧 `**该文章为付费专栏内容**` 行后调 `parse.AddPaidNotice` 前置新引用块。
  附 `20260620_test.go` 覆盖前置/strip/幂等/空 text。记录 命中/补写/跳过/失败 计数日志（仿 20260612）。
- 触发：**已改用迁移注册表**（合并 master 的 migration-registry 框架后）。`init()` 里
  `Register(Migration{Version:20260620000000, Name:"zhihu-paid-notice-backfill", Auto:true})`，
  启动时由 `RunAuto` 自动执行；`Run` 返回 error（任一行更新失败则不记录、下次启动重试）。
  不再加专属控制器方法与 `POST /20260620` 路由（统一走 `registry`/`run/:version`/`run-pending`）。

`just lint` + `go build ./...`。提交 `feat(zhihu): migration 20260620 backfill paid notice into text`。

### 步骤 6：收尾

- 跑改动相关的 targeted 测试（parse 包）；`just lint` 全绿。
- 更新 [docs/PROGRESS.md](PROGRESS.md) 状态为「实现完成，待 review / 部署」。
- 请作者 review；批准后 squash 合并 master、删分支（按 AGENTS.md 流程）。
- 部署后手动 `POST /api/.../migrate/20260620` 跑一次回填（线上），抽查 canglimo 文章
  与一条付费答案，确认 RSS/archive 正文首行为带链接引用块。

## 风险 / 注意

- **避免与 update_at 新鲜度短路冲突**：解析期若 `storedIsCurrent` 命中会直接返回旧
  `Text`（不重烘焙），这是正常的——历史条目由步骤 5 迁移补齐；新抓取/更新走步骤 3。
- **`md.Quote` 多行**：引用块文案是单行，`md.Quote` 仅前置 `>`，安全。
- **strip 旧提示**：仅在行首精确匹配 `**该文章为付费专栏内容**` 时移除，避免误伤正文。
- **migration 只 UPDATE text**：无 schema 变更，AutoMigrate 不涉及；可重复跑。
