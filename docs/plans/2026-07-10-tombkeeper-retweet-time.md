---
title: "转发引用块末尾注明原博时间"
issue: docs/issues/2026-07-10-tombkeeper-retweet-time.md
status: done
areas: [tombkeeper, render]
updated: "2026-07-10"
---

# PLAN: tombkeeper 转发引用块末尾注明原博时间

日期：2026-07-10 · SPEC: [issues/2026-07-10-tombkeeper-retweet-time.md](../issues/2026-07-10-tombkeeper-retweet-time.md)

## 目标

见 issue。转发帖内联的被转发原文引用块末尾追加一行原博发布时间，让读者能判断
转发的是哪天的内容（原博时间此前无处呈现）。

## 关键决策

1. **改动点：retweet 调用处，不进 `renderContent` 通用体。** 时间行只加到「转发 @」引用块
   ([`parse.go:82`](../../pkg/routers/tombkeeper/parse.go)：`renderContent(*orig, nil, 1)` →
   `quoteBlock(...)`)。`orig` 是 `RawPost`，`orig.CreatedAt` 就在手，最短路径。**不**下沉到
   `renderContent` 里按 `depth>0` 统一加 —— 那会波及 `微博正文 N` 内联引用（走
   `materializePost`，返回库里 `TextMarkdown`、拿不到 `CreatedAt`），是另一条路径、非本次所求。

2. **时区：包内固定 +08:00，不用 `config.C.BJT`。** 用
   `var cst = time.FixedZone("CST", 8*3600)`。理由：① `config.C.BJT` 在本包单测里是
   **nil**（测试只赋 `config.C.Settings.ServerURL`，从不 init BJT），`.In(nil)` 会 panic 掉
   现有 render 测试；② 微博时间恒在 1991 年后，中国无夏令时，`FixedZone("CST",8h)` 与
   `LoadLocation("Asia/Shanghai")` 对任何微博时间**完全等价**，且零依赖（不吃 tzdata、不吃
   config init）。`orig.CreatedAt` 是 `$D…Z` 解析出的 UTC instant，`.In(cst)` 转北京墙钟。

3. **格式：`2006 年 01 月 02 日 15:04`**（与 issue 示例一致，全角「年月日」+ 空格 + 24h HH:MM）。

4. **零时间跳过。** `orig.CreatedAt.IsZero()`（`parseFlightTime` 解析失败的回退值）时**不**加
   时间行，避免渲染 0001 年。与 `buildPost` 对顶层帖零时间回退 now 的容错同源、但引用块里
   宁缺毋错。

## 改动

- `pkg/routers/tombkeeper/parse.go`：
  - 加 `var cst = time.FixedZone("CST", 8*3600)` 与
    `func retweetTimeLine(t time.Time) string`（`t.In(cst).Format("2006 年 01 月 02 日 15:04")`）。
  - `renderContent` retweet 分支：`quote := renderContent(*orig,…)` 后，若
    `!orig.CreatedAt.IsZero()` 则 `quote = md.Join(quote, retweetTimeLine(orig.CreatedAt))`，再
    `quoteBlock("转发 @"+orig.ScreenName, quote)`。时间行随 quote 一起被 `md.Quote` 前缀 `>`，
    落在引用块内、原文内容之后。

## 测试（必填）

- `parse_test.go`（或 `render_markdown_test.go`）新增用例：喂 `retweet_with_original.json`
  夹具，断言渲染 body 的转发引用块末尾含 `> 2026 年 06 月 08 日 08:55`（原文
  `$D2026-06-08T00:55:15.000Z` → 北京 08:55），且在原文内容之后。
- 补一条零时间用例：`orig.CreatedAt` 为零 → body 不含时间行、不 panic。
- 跑 `go test ./pkg/routers/tombkeeper/...`。golden `testdata/tombkeeper.atom` 由
  `samplePosts()`（两条硬编码纯文本帖、无转发）生成，**不含转发条目**，本改动不触及 → 预期
  **无需** regenerate（`grep 转发 testdata/tombkeeper.atom` 为 0 即确认）。

## 待更新文档

- [ ] `docs/PROGRESS.md`：完成时追一条（改动 + 发版结论）。
- [ ] `pkg/routers/tombkeeper/example/README.md`（SSOT）：转发内联渲染小节补一句「引用块末尾
      注明原博北京时间」。
- [ ] `testdata/tombkeeper.atom`：无需改（快照由 `samplePosts()` 生成、无转发条目）；仅核对
      `grep 转发` 为 0。
- [ ] `reorder.go`：确认 `ReorderInlineQuotes`（历史一次性迁移）不受影响 —— 时间行在
      `> 转发 @` 块内末尾，不匹配其 `转发`/`微博正文` 块头正则，无需改；仅核对，不动代码。

## 后续项

- `微博正文 N` 内联引用是否也加原博时间：本次不做。该路径走 `materializePost`，时间**能**拿到
  （库里 `Post.PostTime` / 抓取的 `RawPost.CreatedAt`），只是要把它透传出返回签名 —— 是额外
  改动、非本次所求。一致性理由成立（自引微博也可能是旧帖），若认为该加，另开 issue。
