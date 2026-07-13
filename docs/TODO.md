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

`go test ./internal/controller/archive/` 的 `TestExtractAnswerID` 也是**预存**失败（在 master 上即可复现，非 archive-format 改动引入）：`ExtractAnswerID` 早已改为返回 `*zhihuAnswer`，但该用例仍按旧签名拿它和字符串 `answerID` 比较，必然 not-equal。~~修复方向：把用例改成 `result.answerID`（或 `strconv.Itoa(result.answerID)`）再断言。~~ **已于 2026-07-10 修复**（`feat-tkblog` 分支搭车提交 `957aeae`：用例 `Output` 改为 `*zhihuAnswer` 结构断言）。

`go test ./pkg/routers/endoflife/` 的 `TestParseCycles` 也是**预存**失败（在 master 50e088c 即可复现，非 unified-rss-pipeline 改动引入）：用例期望 `ParseCycles` 返回 3 条 versionInfo，实际 `filterCycles` 只保留 `latest==最新版` 或 `lts` 的 cycle，返回 1 条。需确认是 fixture/期望陈旧还是 `filterCycles` 过滤逻辑回归。

## 知乎 / 星球事实化渲染后续（zhihu-zsxq-fact-rendering）

来源：2026-07-12 [plan](plans/2026-07-12-zhihu-zsxq-fact-rendering.md) 明列的后续项，本次重写有意不做。

- **对象存储 staging / 不可变 key 与 orphan 回收**：抓取期转存的图片/附件文件**不入事务**（失败留
  MinIO 未引用文件，同稳定 key 可复用/覆盖）。本次刻意不引入 staging 目录、不可变 key 或 orphan 回收；
  若未引用文件积累变痛，再单开。两源共同。
- **`raw` → typed `content` 事实列（YAGNI 备选）**：读取期每次从 `raw` 经 `parse/models`/`api_models`
  反序列化装配快照。若将来上游 API schema 漂移导致「按老结构反解旧 `raw`」变脆/变痛，可把稳定字段物化
  成 typed `content` 列（而非继续依赖即时反解）。当前 `raw` 反解够用，属 YAGNI 备选，两源共同。

## 统一响应格式后续（unified-response-format）

- **`/statistics` 页面在统计数据为空时白屏崩溃**（预存 bug，与统一响应格式无关）。webapp 的 `<ActivityCalendar>`（react-activity-calendar）在收到空数据时抛异常，且无 error boundary，导致整页不渲染。复现：当 canglimo 在"过去一年"窗口内没有回答时（如本地 DB 数据陈旧），`GET /api/v1/archive/statistics` 返回空 `data`，前端解包为空 → 组件崩溃。生产环境有近一年数据故正常。修复方向：`StatisticsPage`/`Statistics` 组件加空数据 guard（空时显示占位/提示），或包一层 error boundary。
- **后续 API 设计议题**（首性原理讨论结论，留作后续 SPEC）：读操作 POST→GET、content_type 字符串/整数枚举统一、bookmark 子资源化（`PUT/DELETE /topics/{id}/bookmark`）、`/sub/sub/zhihu` 笔误路径修复。

## tombkeeper 订阅源后续

来源：2026-06-18 tombkeeper-rss 实现后的对抗式评审（确认项已修复，下列为有意延后的加固/增强）。

- **图片抓取的纵深 SSRF 加固（低优先）**：已对「完整 URL 形态的 `pics`」加 host 允许名单、并约束图片下载的重定向 host。残留风险（评审指出）：被允许的 CDN 若 302 到内网、或 DNS rebinding。彻底防御需在拨号层（`net.Dialer.Control`）阻断私有/环回/链路本地网段（10/8、172.16/12、192.168/16、127/8、169.254/16、::1、fc00::/7）。鉴于上游 `tombkeeper.io` 单一可信、且为 blind SSRF（响应直接入 OSS 不回显），按低优先处理。
- **用 LLM 生成 tombkeeper 标题（SPEC 后期项）**：当前标题由读取端从正文前 10 个 rune 纯函数推导，
  数据库不保存 `title`。若要引入 LLM，应把模型输出建成独立、带来源/版本的内容事实，再由读取端选择；
  不要把它塞回 Markdown 缓存或覆盖源正文。
- **被内联帖的 `url_info` 缺失**：嵌入的转发原文对象有时不带 `url_info`（其 `t.cn` 短链不展开，正文里保留裸 `t.cn`，由 GFM 自动链接）。如需完整展开，可对原文补一次详情抓取，但会增加请求量，暂不做。
- **微博换行被连排（East Asian Line Breaks 的既有行为，与 A6 无关）**：tombkeeper 正文把多行微博用单 `\n` 拼接（[`render_markdown.go:46`](../../pkg/routers/tombkeeper/render_markdown.go) `strings.Join(lines, "\n")`），而 feed 的 goldmark（A6 后 CSS3Draft、A6 前 `extension.CJK`/Simple **同样**）会去掉 CJK 软换行，导致多行微博在 RSS/归档 HTML 里**连排成一行**。这是 East Asian Line Breaks 扩展的设计行为（避免 CJK 软折行在 HTML 里渲染成字间空格），A6 前后**一致**、非本次引入。若要让微博换行**可见保留**，应在**解析层**把微博的 `\n` 转成硬换行（行尾两空格 `\n`）或段落 `\n\n`（改 `escapeMarkdown`），而非改 goldmark 配置。取舍：硬换行会让每条多行微博每行都带 `<br>`，需先确认这是期望呈现。来源：2026-06-24 unified-rss-pipeline A6 讨论。

## tkblog 博客（xfocus / baidu）

来源：2026-07-10 tkblog 实现（[plan](plans/2026-07-10-tkblog-xfocus-baidu.md)）。

**永久非目标（作者 2026-07-10 定，不是延后 —— 别再当 TODO 捡起来）**：RSS/Atom 出口、cron 增量、
断点续传。理由：博文内容已定型、篇数少，按需全量幂等伪 job 足矣。

**真·延后（有用非必需，将来可做）**：

- **归档额外认 wayback URL**：当前只认粉丝站链接 `tombkeeper.io/{category}/{id}`。如需用 wayback
  原文链接也能归档，按 `source_url` 反查库加一个匹配分支。
- **flight 解析三家共享**：`pkg/routers/tkblog/extract.go` 复制了微博的 flight 机件（已带 `ponytail:`
  注释）。若再出现第三个 Next.js 源，抽到 `internal/nextflight` 共享。

## tombkeeper 评审修复（2026-06-23）

来源：2026-06-23 对 `feat-tombkeeper-rss` 分支的 xhigh 代码评审。下列为已确认的修复项及商定方案，逐条一提交，完成即勾选。

- [x] **#8 图像抓取限流（option B）**：新增独立 pic 限流器（0.25s + 0.25s 抖动），在 `downloadFirstAvailable` 每张图探测**前**取一个令牌；单图内仍 16 路并发取首个成功，图间 0.25–0.5s。`Requester` 接口加 `WaitPicSlot()`。
- [x] **CDN 探测取消/提前返回**：`GetPicStream` 改为带 `context.Context`（`NewRequestWithContext`），`downloadFirstAvailable` 拿到首个成功即 `cancel()` 中止其余在途请求并立即返回，后台 drain 关闭竞速到的 body。修复「等最慢候选（最坏 ~20s）才返回」。（每个候选用独立 context，仅取消败者；胜者 context 随其 body 关闭而释放——`cancelOnClose`。）
- [x] **#7 SSRF 加固**：主 `client` 加与 `picClient` 同款 `CheckRedirect`（≤5 跳 + 重定向目标 host 校验），覆盖 `GetReppic`/`GetDetail`/`GetPage`。共享 `redirectGuard(allow)` 助手，页面客户端用 `pageHostAllowed`（tombkeeper.io / *.weibo.com）。
- [x] **#1 BidToMid 越界 panic**：非最左组解码值 >7 位（≥10^7）时返回 error，而非 `strings.Repeat` 负数 panic。不引 base62 库（分组逻辑为 weibo 专有，库无益）。
- [x] **#2 微博归档路由 + uid 校验**：tombkeeper 包导出唯一匹配器 `WeiboArchiveMid`（预编译正则，uid/bid 形态校验 `IsTombkeeperUID`），派发器三 case 合一、未命中落 unknown link；删重复的 `tombkeeperMidFromLink`。测试随匹配器迁到 tombkeeper 包（`TestWeiboArchiveMid`，有序、含非本人 uid 拒绝用例）。
- [x] **#12 未找到返回 404**：archive 包加哨兵 `ErrArchiveNotFound`，`HandleTombkeeperWeibo` 命中 `gorm.ErrRecordNotFound` 时 `%w` 包它，`History` 用 `errors.Is` 映射成 404。
- [x] **#13 OSS 保存失败无记录丢图**：`SaveStream` 失败时退化到放弃记录（`status=Abandoned`、`object_key` 空、`URL=原链`），返回原链而非 error；正文保留图 + 落记录 + 下次跳过。
- [x] **#4 列表/表格不转义**：行首正则扩 `-`/`+`/`*`/`数字.`（列表），inline replacer 加 `|`（GFM 表格）。`数字.` 仅在其后为空白/行尾时转义（不动 `3.14`）。
- [x] **#6 时间解析失败记当前时间**：在 `buildPost` 处理（`parseFlightTime` 失败返回零值，零值即解析失败信号）。`CreatedAt` 为零时改用 `time.Now()` 并 warn（带 post id，用 Renderer 已有 logger），避免存成 year-0001 污染 feed。
- [x] **#10 objStartRe 漂移可见化**：正则容忍空白 `\{\s*"id"\s*:\s*"\d`（加 whitespace 用例）。抓取层对账并入 #11。
- [x] **#11 timelineIDs 两源对账**：Crawl 里 `timelineIDs==0` 而 flight 有对象 → 记 error；timelineID 不在 pageMap → warn（含 #10 的「抽取缺失」对账）。pageMap 多出的对象是内联转发原文，属正常、不告警。放宽兼容 `/weibo/{bid}` 留作后续（已在 TODO「tombkeeper 订阅源后续」语境）。
- [x] **#15 导出 FeedSize 常量**：`tombkeeper.FeedSize = 30`，crawl.go 与 controller/tombkeeper/rss.go 共用。
- [x] **重构（复用/去重）**：goldmark 配置抽 `render.NewMarkdown()`（md2html/md2text/RSS 三处共用）；`imageEmbeds` 用 `md.Image`；删死字段 `RawPost.VideoURL`（恒空、无人读）——**`Post.VideoURL` 实为有效数据**（`videoLink(url_info)` 填充、有测试），保留；`Object.URI()` 抽 `internal/file.ObjectURI`，tombkeeper 与 zsxq 共用（zsxq 的 `ErrNoStorageProvider` 改为别名，保留其测试）。
- [x] **测试完善**：补 `BidToMid` 越界用例（`AZZZZ`）；`fakeDB.GetLatestPosts` 按 `PostTime` DESC 排序 + 截断（贴近生产）；路由错误分支 + 有序断言已随 #2 的 `TestWeiboArchiveMid`（含非本人 uid 拒绝）、#4 的转义用例、#10 的 whitespace 用例落地。

已确认不修：图片放弃后永久固定原链（#5，预期行为）；视频 `strings.Contains` 去重（#9）。
