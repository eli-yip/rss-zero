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

## 补充（2026-07-10）：历史回填 migration

渲染改动只对**新抓**的帖生效；库里已存的转发帖 `text_markdown` 缺时间行。仿
`20260709000000`（inline-quote 重排）做一个**纯离线、无网络**的注册型 migration 回填。

**关键事实（实测夹具）**：转发帖的 `raw`（bytea，逐字节存的原始对象）里**嵌有完整原文**
`retweet_weibo`，含 `created_at`（`$D…Z`）。三个转发夹具（with_original / view_pic /
original_absent）**均存在** `retweet_weibo.created_at` → 原博时间可**离线**从本帖 `raw` 取得，
无需查原文行、无需联网。且它与渲染路径用的 `orig.CreatedAt`（同页独立对象）是同一条微博、
时间戳一致，故回填结果与新渲染**逐字节相同**。

**实现**：

1. `pkg/routers/tombkeeper/reorder.go`（同族「离线 stored-body 字符串手术」，复用 `reRetweetHead`）加：
   - `retweetOriginalCreatedAt(raw []byte) time.Time`：`json.Unmarshal` 取 `retweet_weibo.created_at`
     → `parseFlightTime`；缺失/解析失败返回零值。
   - `AppendRetweetTime(body string, raw []byte) string`：`reRetweetHead` 无匹配 / 原博时间零值 →
     原样返回；否则按 `reorder.go` 的 `Split("\n\n")` 取**转发块** `blocks[0]`（其内无 `\n\n`），
     在其末尾追加 `"\n> \n> " + retweetTimeLine(t)`（与渲染实测字节一致：空引用行为 `>`），
     再 `Join` 回去。已含该时间行（`HasSuffix(retweet, "> "+line)`）则原样返回 → **幂等**。
     取转发块而非直接 append body 尾：不依赖「转发块在最后」，即使某帖未经重排也只动转发块。
2. `internal/migrate/20260710000000.go`：注册 `Migration{Version:20260710000000, Name:
   "tombkeeper-retweet-original-time", Auto:true}`，`text_markdown LIKE '%> 转发 @%'` 收窄扫描，
   逐条 `AppendRetweetTime(p.TextMarkdown, p.Raw)`，变了才 `Update`，记 `scanned/updated`。
   版本号 > `20260709000000`，注册表按版本升序跑，重排在前、本回填在后。

**测试**：`AppendRetweetTime` 单测 —— 取新渲染 body 去掉时间行后缀当「旧 body」，断言
`AppendRetweetTime(旧, repost.Raw) == 新渲染 body`（逐字节）、再跑一次幂等、无转发块原样返回、
`raw` 无 `retweet_weibo` 原样返回。（migration 壳是薄循环、仿已验证的 `20260709000000`，不另测。）

**待更新文档（补充）**：

- [ ] `docs/OPS.md`：迁移清单补 `20260710000000`（若有该清单）。
- [ ] `docs/PROGRESS.md`：发版结论里带上「启动自动迁移 scanned/updated」。

## 后续项

- `微博正文 N` 内联引用是否也加原博时间：本次不做。该路径走 `materializePost`，时间**能**拿到
  （库里 `Post.PostTime` / 抓取的 `RawPost.CreatedAt`），只是要把它透传出返回签名 —— 是额外
  改动、非本次所求。一致性理由成立（自引微博也可能是旧帖），若认为该加，另开 issue。
