# SPEC: /archive 多格式输出（HTML / Markdown / 纯文本）

- 日期：2026-06-17
- 状态：已实现，待 review

## 背景

`GET /api/v1/archive/:url` 把站点（知乎答案/文章/想法、知识星球主题）的归档内容
渲染为完整 HTML 页面直接返回。内部链路为：DB 取内容 → full-text render 生成
**Markdown 全文** → goldmark 转 HTML → `c.HTML` 返回。

Markdown 全文是真正的「单一数据源」，HTML 只是它的一个渲染产物。目前接口只暴露
HTML，缺少机器/脚本友好的纯文本形态。

## 目标

为该接口增加两种输出格式，且**完全兼容现有调用方**：

1. **Markdown**：直接返回 full-text 的 Markdown 原文（含标题/作者/时间/链接）。
2. **纯文本（txt）**：在 Markdown 基础上去除所有 Markdown 标记后的可读纯文本。
   去标记后会**折叠夹在两个汉字之间的空格**——`文本 **强调** 文本` 里那对空格只是
   markdown 的语法辅助（部分解析器要求强调标记两侧有边界），去掉标记后是多余的；
   紧邻非汉字（如 `调用 foo()` 的拉丁文）的空格保留不动。
3. **HTML**：保持现状，为缺省格式。

## 请求格式

通过 `format` query 参数选择：

| 请求                                  | 返回             | Content-Type                   |
| ------------------------------------- | ---------------- | ------------------------------ |
| `GET /api/v1/archive/:url`            | HTML 页面        | `text/html`                    |
| `GET /api/v1/archive/:url?format=md`  | Markdown 全文    | `text/markdown; charset=utf-8` |
| `GET /api/v1/archive/:url?format=txt` | 去除标记的纯文本 | `text/plain; charset=utf-8`    |

- `format` 取值（大小写不敏感）：`md`/`markdown`、`txt`/`text`/`plain`、`html`。
  **未知值或缺省一律按 HTML 处理**，保证向后兼容、不会因拼错而报错。

## 跨格式跳转（格式切换导航）

三种输出在正文顶部都带一行「格式切换」导航，便于在 HTML / Markdown / 纯文本
之间互相跳转：

- **HTML / Markdown**：导航为一行 markdown —— 当前格式加粗不带链接，另两种为
  指向 `?format=` 的链接，后跟一条分隔线（HTML 渲染为 `<hr>`）。
- **纯文本**：导航为纯文本行（**不含 markdown 标记**），但保留另两种格式的完整
  URL，确保在纯文本里仍可复制跳转。

导航链接里的源 URL 用 `url.PathEscape` 转义，保证作为单个 `:url` 路由段能正确
回环（与 handler 的 `PathUnescape` 对应），比旧 `BuildArchiveLink` 的未转义拼接更稳。

> 注：因新增导航行，HTML 输出在正文顶部多了一行格式切换 + 分隔线，正文内容本身不变。

## 兼容性约束（关键）

现有逻辑：`:url` 上**只要带任意 query 参数即 302 重定向**到干净归档链接
（用于剥离源链接上的 tracking 参数）。新机制必须与之共存：

- `format` 参数从「带 query 即重定向」的判定中**排除**；
- 其余 query 参数仍按原逻辑触发重定向；
- 若在重定向时同时带了非缺省 `format`，重定向目标会**保留** `?format=`，不丢失意图。

## 非目标

- 不改变 HTML 的样式与内容。
- 不引入 Accept 头内容协商（已与作者确认采用 query 参数，浏览器/curl 更易用）。
- 不改变 `POST /api/v1/archive` 等其它路由。
