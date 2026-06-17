# PLAN: /archive 多格式输出

- 对应 SPEC：[specs/2026-06-17-01-archive-format-negotiation.md](../specs/2026-06-17-01-archive-format-negotiation.md)
- 分支：`feat-archive-format`

## 设计要点

把 HTML 渲染从各 `Handle*` 处理器**上移到 `History`**，让处理器只产出
「单一数据源」`{title, markdown}`，由 `History` 按 `format` 决定最终渲染。
这样三种格式共用同一条取数链路，互不重复。

## 步骤

1. **新增 Markdown → 纯文本转换** `pkg/render/md2text.go`
   - `Markdown2Text(content string) (string, error)`：用 goldmark 解析为 AST 并遍历，
     只取各节点文本载荷——标题/链接/强调去标记留文字、代码块保留字面、行内代码去反引号、
     图片退化为 alt、raw HTML 丢弃；块级节点间用空行分隔，末尾折叠多余空行。
   - 复用与 HTML 渲染相同的扩展（GFM + CJK），保证解析一致。
   - 单测 `md2text_test.go` 覆盖标题/强调/链接/列表/代码块/行内代码/图片。

2. **重构 `archiveResult`**（`internal/controller/archive/archive.go`）
   - 字段由 `html` 改为 `title, markdown`（保留 `redirectTo`）。
   - 新增 `format*` 常量与 `parseArchiveFormat`（未知值落到 HTML）。

3. **改造各处理器**返回 `{title, markdown}`，删除其中的 `htmlRender.Render` 调用：
   - `zhihu_archive.go`：Answer / Article / Pin
   - `zsxq_archive.go`：ZsxqWebTopic（重定向分支不变）

4. **改造 `History`**
   - 解析 `format`；
   - 重定向判定排除 `format`，并在重定向时保留非缺省 `format`；
   - 按 `format` 分支：md → `c.Blob(text/markdown)`、txt → `Markdown2Text` 后
     `c.Blob(text/plain)`、缺省 → `htmlRender.Render` 后 `c.HTML`。

5. **跨格式跳转导航**
   - `buildFormatLinks(sourceURL)`：用 `url.PathEscape` 拼出三种格式的归档链接。
   - `formatSwitcherMarkdown(current, …)`：一行 markdown，当前格式加粗、另两种为链接
     （供 HTML 与 Markdown 输出用，顶部 + `---` 分隔线）。
   - `formatSwitcherText(htmlLink, mdLink)`：纯文本导航行，保留完整 URL，在
     `Markdown2Text` **之后**拼到纯文本顶部，确保 URL 不被去标记吃掉。

## 验证

- `go build ./internal/controller/archive/... ./pkg/render/...` 绿。
- `go test ./pkg/render/ -run TestMarkdown2Text` 通过。
- `just lint` 0 issue。
- 注：`TestExtractAnswerID` 为**预存**失败（与本次无关），记入 TODO。
