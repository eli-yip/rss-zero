---
title: "知乎与知识星球事实入库与读时 Markdown 渲染重写"
issue: docs/issues/2026-07-12-zhihu-zsxq-fact-rendering.md
status: draft
areas: [zhihu, zsxq, rss, controller, migrate, db, render, file]
updated: "2026-07-12"
---

# PLAN：知乎与知识星球事实入库与读时 Markdown 渲染重写

## 目标

解决 [issue](../issues/2026-07-12-zhihu-zsxq-fact-rendering.md)：把两源的 seam 从「抓取时产出已排版
Markdown 并冻结进 `text`」改成「抓取时只落事实，读取时纯函数渲染」。两源共用同一套模式（tombkeeper
已验证：`docs/plans/2026-07-11-tombkeeper-fact-rendering.md`），故写成**一份 plan**；但内部**分两阶段实施**
——先 ZSXQ pilot（事实完整、blast radius 最小）把模式跑通，再 Zhihu（3 类型 + 付费 + question-FK 等更硬
的边角）按 delta 复制。

## 当前症结

`render.Text` / `ParseAnswer` 等是被放在**写路径**上的浅模块：调用方只想保存内容，却顺带做 HTML→MD、
OSS 换链、付费提示、块/引用排版、格式化，产物冻进 `text`；读取侧（`fetch_{zhihu,zsxq}.go`、archive、
export、random）只给冻结正文套壳。展示规则一变就得改抓取 + 全库 refmt 回填。此外资源先写、内容根行
后写，中途失败留半态。

关键观察：**正文所需的一切都已作为事实存在**——`raw`（原始载荷）+ 侧表（zhihu_object/question、
zsxq_object/article/author）。`text` 只是这些事实的一份冻结副本。所以是**原地删列**，不 truncate、不重抓。

## 领域语言与类型名

类型名取内容概念，不取「解析事实」「准备渲染」等处理步骤；包名已提供来源语境，不给类型统一加
`Fact`/`Record`/`Document`。持久化数据分三类（沿用 raw-SSOT 划分）：**来源载荷**（`raw`，一切正文渲染
的起点）· **查询投影**（id/作者/问题/时间/类型/精华/标题/`word_count`——**仅当有 SQL 查询谓词/排序需要
时才物化成列**，如 `word_count` 供 random 的 `between 300 and 1200` 过滤；**纯派生而无查询需求的一律读取期
计算、不存列**，如付费 paid，见决策 5。这不违反 SSOT：物化列是 raw 的单向缓存、重建方向固定
`raw → 投影 → Markdown`，而无查询的冗余存储才是 SSOT 违背）· **外部处理结果**（AI 标题、检测状态、
embedding、Object 转存信息、语音转写，无法仅靠 raw 稳定重现，单独保存）。

| 概念                     | Go 名称                                     | 不采用                           |
| ------------------------ | ------------------------------------------- | -------------------------------- |
| 一次呈现所需的自包含内容 | `ContentSnapshot`（不持久化）               | `RenderDataset`、`RenderContext` |
| 内容快照装配模块         | `ContentLoader`（各源一个）                 | 跨源 `Library`                   |
| 内容只读 adapter         | `ContentReader`                             | `FactReader`                     |
| 待提交的一条解析结果     | `AnswerParseResult` / `TopicParseResult` 等 | 通用 interface                   |

纯 renderer 唯一环境输入命名为 `serverBaseURL string`，不用开放式 `RenderOptions` 参数袋。

## 关键决策

### 1. `raw` + 侧表是事实来源；**原地删 `text`，不 truncate、不重抓**（已实测）

保留内容表与 `raw`，删 `text`；不新增 `content_html`/`parts`/通用 AST。读取期由 `ContentLoader`
反序列化 `raw`（经既有 `parse/models`/`api_models`）并装配侧表事实。两源 refmt 早就从 raw 重渲染，证明
raw 充分。

**为何原地删列而非 tombkeeper 的 truncate+ 重抓**：tombkeeper 必须 truncate 是因为其 flight `raw` 不可
独立重解析；知乎/星球的 `raw` 可重解析，且付费/删除内容**不保证可重抓**，truncate 会永久丢掉这类不可再生
数据。**线上实测（2026-07-12，`rss-db` 只读计数）**：`raw` content 为空的行——zhihu_article **0/1794**、
zhihu_pin **0/8321**、zhihu_answer **18/24586**，且这 18 行 `text` 本就只有一个 `\n`（本来就空白）；
zsxq 事实存 raw + 侧表同样完整。即「content 空 ⟺ text 本就空」，原地删列**零丢失、零行为变化**。开工时
复跑此审计确认增量成立即可（期望仍个位数本就空白行）。

`raw` 保留、不删（与 tombkeeper 不同），它是唯一无损事实源、删了不可逆。

### 2. 读取期：`ContentLoader` 批量装配 → 纯 `RenderMarkdown`，四路共用

各源一个具体 loader（不建跨源 Library），两阶段批量装配自包含快照，避免 N+1：

```go
RenderMarkdown(id int64, content ContentSnapshot, serverBaseURL string) (string, error) // 纯函数
ContentLoader.Load(roots []Root) (ContentSnapshot, error)                                // 注入 ContentReader
```

- 知乎快照含 Answer/Article/Pin 根行 + `Questions`（answer 标题）+ `Objects`（图片换链）；`origin_pin`
  已内嵌在 pin `raw`，自包含。
- 星球快照含 Topic 根行 + `Objects`（图片/文件/语音 + 转写）+ `Article`（外部文章）+ `Author`。
- feed 最多 50 roots 共用一次装配；archive 单 root、export 分页、random 只改选择条件与 identity——
  四路**共用同一 body renderer**，各自只保留 envelope/footer/摘要/identity 差异。
- loader 可查库；renderer 只收自包含输入，不查 DB/不联网/不调 AI/不读配置。OSS URI 由快照里的 Object
  事实（`object_key`+`storage_provider`）在 renderer 内经 `internal/file.ObjectURI` 拼算（纯拼接，不读
  配置）。CJK 统一走读取期 `EastAsianLineBreaksCSS3Draft`（`pkg/render/feed_markdown.go`，遵守 MEMORY
  「CJK 换行约定」，合并软换行、不自造硬换行）。

### 3. 抓取期只落事实；派生事实由 **transient 纯渲染**喂入、不持久化

parse 去掉一切**持久化正文**的渲染与 `Text` 写入，保留事实抽取（作者/问题/对象/外部文章）。

抓取期停止**保存** Markdown 后，仍有派生事实当前从已排版正文算：知乎 `word_count`
（`answer.go:135`，闸 random `between 300 and 1200`，`db/answer.go:271`）、内容检测
`detector.Detect(authorID, formattedText)`（写 `detect_status`，闸 RSS 可见性）、embedding
（`saveEmbedding`）、两源 AI 标题。**做法（吸收 Codex）**：新 `RenderMarkdown` 是纯函数，抓取期用内存装配
的 `ContentSnapshot` **调它一次**、拿临时 Markdown 喂这些派生事实，**但不持久化**——派生事实与今天逐字节
一致（`word_count`/`detect_status` 不漂移、random 成员与 RSS 可见性不变），且调的就是读取期那一个纯函数、
没把写侧 renderer 拉回来。

即「抓取期不**保存** Markdown」是硬约束；「抓取期不**生成** Markdown」不是——纯函数即用即弃更安全。若将来
要连生成都禁掉，须单独重定义 AI/检测/word_count 输入，不属本次。freshness fast path（`storedIsCurrent`）
保留：unchanged 在任何外部副作用前结束；新 parser 不再返回已存 `Text`，调用方只消费 unchanged 信号。

### 4. 保存原子化：Object/Author/Question/Article + 根行同一事务（吸收 Codex，修既有 bug）

现在资源先写、内容根行后写（`zhihu/parse/image.go`、`zsxq/parse/talk.go`），中途失败留可见半态。改为
parse 返回**待提交结果**（`AnswerParseResult{Answer, Question, Objects}` / `TopicParseResult{Topic,
Author, Article, Objects}` 等，各源按需定义、不抽通用 interface），保存模块在**一个 GORM 事务**里写完
本条产生的全部行、**根行最后写**；embedding 仍在事务提交后 best-effort。对象存储文件不入事务（失败留
MinIO 未引用文件，同稳定 key 可复用/覆盖，不引入 staging/orphan 回收——列后续项）。

### 5. 两源差异（同一模式下的 delta）

**知乎（Phase 2）**：

- HTML→Markdown 从抓取期移到读取期纯 renderer；**只动 zhihu 专属规则**（`render/html_convert.go` 的
  `figure`/`data-original`），不碰共用 `getDefaultRules`（否则移动别源 golden）。成本由 Redis feed 缓存吸收。
- 付费**读取期从 raw 推导、不存列**（SSOT 修正）：`IsPaidAnswer(answer_type)` /
  `IsPaidArticle(article_type, paid_info)`（`parse/paid.go`）本就是 raw 字段的纯函数，且全仓 grep 确认
  **无任何 SQL 谓词按 paid 过滤/排序/计数**——故把这两个纯函数移进读取期 renderer、按需渲染付费提示即可，
  **不新增 `is_paid` 列、不回填**。（对比 `word_count` 之所以物化成列，是因为 random 选答有
  `between 300 and 1200` 的 SQL 过滤：有查询谓词才物化，纯派生无查询需求的一律读时算。）
- 图片换链移到读取期：抓取期仍下载转存写 `zhihu_object`，但不再 `replaceImageLink`；renderer 按
  `URLToID(url)` 查快照 Object 拼 OSS URL，查不到降级（保留原 URL，对齐现 `OfflineImageParser` 缺行行为）。
- pin 块模板与一层 `origin_pin` 引用移到读取期；`ServerURL` 改由 `serverBaseURL` 显式入参。
- **question-FK**：answer 标题取自单独的 `zhihu_question` 表（外键），快照批量装配 questions；缺失时
  **显式降级**（标题回退到问题 id / 固定占位），不再像 `fetch_zhihu.go:73` 那样让整个 feed 构建失败。
- `detect_status` 已是结构化列，读取期 `GetLatestNVisibleAnswer` 过滤，不动。

**星球（Phase 1，pilot）**：

- 事实已在 raw + `zsxq_object`/`zsxq_article`/`zsxq_author`，**零回填**，只删 text。侧表结构不变
  （object 的 `topic_id` 关联无需动——原地删列不重建）。
- **外部文章 HTML→Markdown 归一化是唯一豁免、保留在抓取期**：它抓的是独立外部文档（`saveArticles`
  联网抓 HTML），转换是对该文档的有界事实规范化，产物存 `zsxq_article.text`（一条 article 事实，非 topic
  正文），topic renderer 读取期只内联它；把 converter 留抓取期也让其超时/pathological-HTML 风险不外溢到
  读取期 feed（这正是 `topicIDSkip` 存在的原因，故它随之留抓取期）。`zsxq_article.raw` 保留可日后无损移读。
- **未知 type**（非 talk/q&a 的 default 分支）**会**以空 `text` 存 metadata+raw，读取期由
  `zsxqRender.Support(type)` 过滤（保持现状）；`topicIDSkip`/被屏蔽作者/无正文 talk 在 `SaveTopic` 前
  return、从不入库（保持现状）。

### 6. 行为契约矩阵（吸收 Codex，作回归清单）

以矩阵钉死当前用户可见的 skip/error/save 行为，逐行配测试（评审确认对代码准确）：

| 来源内容                       | 条件                                                     | 现有行为（本次保持）                                              |
| ------------------------------ | -------------------------------------------------------- | ----------------------------------------------------------------- |
| 知乎 Answer/Article            | HTML 为空                                                | 存 root；读取渲染空正文 + 既有 wrapper（线上仅 18 条已空 answer） |
| 知乎 Pin                       | 内容项 + Origin 合并后仍空                               | skip，不存 root                                                   |
| 知乎 Pin                       | 未知内容项                                               | error，不存 root                                                  |
| 知乎 Pin                       | poll 无可见正文且无其他内容                              | 按空 Pin skip                                                     |
| 知乎 Answer                    | detect 命中隐藏                                          | 存 root；读取期 `GetLatestNVisibleAnswer` 过滤                    |
| 星球 Talk                      | Talk/Text 缺失 / blocked author / 文章 converter timeout | skip，不存 root                                                   |
| 星球 QA                        | Question 或 Answer 缺失                                  | error，不存 root                                                  |
| 星球 QA                        | Answer 文本空但 Voice/Image 有效                         | 继续处理并保存                                                    |
| 星球 Topic                     | `topicIDSkip` 命中                                       | skip，不存 root                                                   |
| 星球 Topic                     | Type 非 Talk/QA                                          | 存 metadata+raw；读取/RSS 过滤                                    |
| 支持类型的其余非既有 skip 错误 | —                                                        | error，不存 root                                                  |

列表分页的 skip/error/成功计数与 target-time 推进沿用现有 `crawlListPages`/`CrawlGroup` 语义，不发明新 outcome。

### 7. 一次破坏性删列迁移（原地），删旧迁移，缓存隔离

新增自动 migration（下一可用版本号，`Auto=true, RequiresPredecessors=true`）。幂等门控 = legacy 列存在：
`if !HasColumn(legacyModel, "text") { return nil }`。满足则在一个 transaction 内对五张内容表
`ALTER TABLE ... DROP COLUMN IF EXISTS text`——五表统一、纯删列，**无新增列、无回填**（付费读时推导，见决策 5）。**不 TRUNCATE**——事实完好，
删列后读取期即可从事实重放。可重试：legacy guard 保证「body 已提交但 runner 记账失败」重启只 no-op。

**旧迁移直接删除**（作者放行：过去的迁移可按需删/跳过，不为编译保留）——比 tombkeeper 的 no-op-guard、
比 Codex 的 legacy-bootstrap 都简单：

- ZSXQ：`20240929.go`（用 `topic.Text`+`GetTopicForMigrate` 回填标题）连同 `GetTopicForMigrate`
  （`db/topic.go:154`，仅此一处调用）与其端点删除。
- Zhihu：`20260620.go`（付费提示回填，调 `AddPaidNotice`）、`20250530.go`（`aiService.Embed(answer.Text)`，
  删 `Text` 会编译失败——评审补漏）连同端点删除。
- 都是一次性历史回填：prod 早已 applied（删注册不重跑，runner 只跑未 applied），fresh DB 上对新 schema
  无意义；删注册对 append-only registry 安全（`predecessorsDone` 忽略未注册版本）。与「删 `Text` 字段」落
  同一提交，避免编译中断。

两源 RSS/random 换新 Redis source cache key，隔离旧 `text` 生成的 canonical items；crawl warm 与
controller cache-miss 同时改 key。不可回滚到读 `text` 的旧二进制；回滚手段是 DB 备份。

### 8. 删除两源 refmt

删 `pkg/routers/{zhihu,zsxq}/refmt`、`internal/controller/{zhihu,zsxq}/refmt.go`、两条路由注册及构造依赖/
通知/测试；保留 xiaobot refmt。读时渲染后「反解 raw 重写 text」失去意义；规则变化下次读取/cache miss 自然
作用于既存 `raw`，无需数据库重写。

## 代码落点

- `pkg/routers/{zhihu,zsxq}/parse/*`：解码/验证/资源处理，返回待提交结果；去持久化正文渲染与 `Text` 写入；
  派生事实由 transient 纯渲染喂入。知乎另去 `replaceImageLink`/`AddPaidNotice`（付费判定移读取期 renderer）。
- `pkg/routers/{zhihu,zsxq}/db/*`：删 `Text`；保留 `Raw` 与查询投影；删 `GetTopicForMigrate`；
  加事务保存入口与批量只读查询 + `ContentReader`。
- `pkg/routers/{zhihu,zsxq}/content.go`（新）：`ContentSnapshot`/`ContentLoader` 两阶段批量装配。
- `pkg/routers/{zhihu,zsxq}/render/*`：唯一纯 `RenderMarkdown`（含各源规则）；ZSXQ `full_text.go` 的
  `FullText` 改为正文调 renderer + 归档壳；删 `MarkdownRenderService` 的 DB/Requester/file 依赖。
- `internal/rss/fetch_{zhihu,zsxq}.go`、`internal/controller/archive`、两源 export/random：从 root+snapshot
  生成 canonical items，共用 renderer，不读 `Text`。
- `cmd/server/echo.go` 与两源 controller：删两条 refmt 路由及依赖。
- `internal/migrate`：原地 `DROP COLUMN text`（五表统一，无新增列）迁移 + legacy guard；**删**
  `20240929.go`/`20250530.go`/`20260620.go` 及其端点。

## 实施步骤（对应提交，内部两阶段）

**Phase 1 — ZSXQ pilot（先落地、证明模式）**

1. **行为基线**：characterization 测试锁住当前 talk/q&a 的 `render.Text`、`zsxq.atom`、full_text/export 输出。
2. **读时装配 + 纯渲染**：`ContentLoader`/`ContentSnapshot`；`render.Text` 迁成纯 `RenderMarkdown`，资源/
   文章/作者从快照取；`format_funcs` 测试保留 + 端到端渲染 fixture。
3. **接入四路读取**：feed/random/archive/export 改 `Load` + 纯 renderer；更新 `zsxq.atom` golden，人工核对
   差异只来自已声明语义等价（作者名改取当前 `zsxq_author`、CJK 统一 CSS3Draft）。
4. **抓取期瘦身 + 原子化**：`ParseTopic` 去持久化正文/去 `Text` 写入、事实抽取入同一事务；AI 标题改 transient
   纯渲染喂入；未知类型/跳过语义订正。
5. **破坏性迁移**：`DROP COLUMN text` + legacy guard + 新 cache key；删 `20240929.go`；runbook。
6. **清理**：删 zsxq `refmt`、`Topic.Text` 及失效测试。

**Phase 2 — Zhihu delta（承 pilot）**
7. 行为基线（answer/article/pin：HTML/图片/付费/空正文/标题/原文链接/pin 六块/origin/word_count/detect）。
8. 三类纯渲染（HTML→MD zhihu 专属规则、付费提示、图片换链、pin 块与一层引用、question-FK 降级）；旧 parse
暂调同一纯 renderer 证明输出不变。
9. 接入 RSS + 归档；更新三份 `zhihu_*.atom` golden。
10. 抓取期瘦身：解析返回待提交结果、object+root 同事务；去 `Text`/去内联改写（付费判定移读取期）；
AI 标题/word_count/检测由 transient 纯渲染喂入。
11. raw 完整度审计（复跑，期望 ~0）→ `DROP COLUMN text`（无新增列）+ 新 cache key；删
`20260620.go`/`20250530.go` 及端点；runbook。
12. 清理 zhihu `refmt`、失效测试；更新架构/OPS/PROGRESS/TODO；跑三路评审。

## 测试（必填）

- **纯 renderer（核心，补盲区）**：直接构造 `ContentSnapshot` 字面量断言两源各类型的 Markdown 语义
  （知乎付费提示/图片换链/pin 六块/一层引用/question 标题；星球作者头/图片文件编号/语音转写/外部文章/
  Q&A 引用/9 道格式化）；编译期不接受 DB/Requester；同输入逐字节一致、输入不被修改。
- **fixture → load → render**：真实单篇/单 topic JSON（含付费、含图、含 origin_pin、含空 content、含语音、
  含外链、含未知类型）跑整条读取路径。
- **行为契约矩阵**：决策 6 每行一测。
- **写入/资源/原子性**：Object+Author/Question/Article+root 同事务成功；同 UpdateAt 重复载荷走 unchanged
  （零资源请求/零文件/零渲染/零 DB 写/零 embedding）；资源或 root 保存失败 rollback（初次无可见行、更新
  保留旧行）；`SaveStream` 失败不推进 Object row，DB rollback 留不可见文件可复用。
- **派生事实一致**：`raw`→`word_count`/`detect_status`/AI 标题与旧值一致（transient 渲染喂入）；random
  eligibility 不变；embedding 失败不影响 answer 其余事实。
- **golden**：`zhihu_*.atom`/`zsxq.atom` 用户可见内容一致 + 新增经真实 `RenderMarkdown` 的 golden。
- **loader**：feed 50 roots 批量装配 questions/objects/article/author，SQL 次数不随条目线性增长；缺失资源
  保持缺失、不触发网络或二次查询。
- **migration/cache**：临时 Postgres 验原地 `DROP COLUMN text`（五表统一、无新增列）、legacy guard 可重试
  no-op、fresh DB 全量 registry（删 `20240929`/`20250530`/`20260620` 后）连续通过；两源 cache-miss 与
  crawl warm 只用新 key；两条 refmt 路由 404、xiaobot refmt 仍在。
- 验证命令：`go test ./pkg/routers/zhihu/... ./pkg/routers/zsxq/... ./internal/rss/...
  ./internal/controller/archive/... ./internal/migrate/...`、`go build ./...`、`just lint`；不跑无关全量。

## 评审（合并前，对 diff 跑，三个 agent 并行，评审者 ≠ 作者）

1. **Standards** —— `/code-review` 标准轴：仓库规范、测试、迁移、文档。
2. **Spec** —— `/code-review` 规格轴：diff 是否兑现本 issue 的目标/验收/行为矩阵。
3. **可读性 Review Agent（作者指定）** —— 上下文严格受限：**只给完整 diff + 核心类型/schema + 每个重点
   函数一个直接 caller/callee + 关键测试和术语表，不给本 issue/plan、作者解释或另两个 reviewer 结论**（拿不到
   设计意图作拐杖，才能检验代码自身是否可读）。重点函数至少：解析入口、事务保存、批量 `ContentLoader.Load`、
   两源纯 `RenderMarkdown`、RSS snapshot→Item 转换、`DROP COLUMN` 迁移。对每个逐项讲清 **流程 · 数据流 ·
   关键中间值 · 数据库状态变换 · 跳过条件 · 每类失败的 retry/continue/abort/fallback 策略**。

   **判定规则**：凡需用「可能/大概/看起来」猜测才能讲清某项之处，即证明该处代码需改（改名/拆函数/显式返回值/
   补一行意图注释），而非补外部文档或大段注释掩盖。产出＝每个函数上述讲解 + 「需猜测处 → 对应代码改动」清单，
   清单非空即挡合并（当场修或开后续 issue）。与 tombkeeper lesson 同取向：读起来要靠猜的代码就是要改的代码。

## 待更新文档

- [ ] `CONTEXT.md`：只补本次真正形成的两源领域词汇（快照/loader/待提交结果），不写存储/迁移/interface 细节。
- [ ] `docs/ARCHITECTURE.md`：数据流「内容库」把 zhihu/zsxq 并入「只存事实、读时渲染」，明确 `raw` 与
      `ContentSnapshot`。
- [ ] `docs/OPS.md`：迁移由框架 `RunAuto` **自动执行、不写手动步骤**；OPS 只记框架不保证的**人为残余**
      ——删列**不可回滚**（旧读-`text` 二进制失效；但 `raw` 未删、源数据不丢，回退＝补回 `text` 列并从
      `raw` 重算）· runner 失败非致命故**部署后 smoke-test 一个 RSS** 确认读-from-raw 正常 · 新 cache key。
      **无手动重抓/回填**（不 truncate，与 tombkeeper OPS 的重量级 runbook 不同）。
- [ ] `docs/PROGRESS.md`：两阶段完成/发版/上线结果与验证数字（与结束分支同一提交）。
- [ ] `docs/TODO.md`：登记「raw→typed content 列」YAGNI 备选、对象存储 staging/orphan 回收后续项。
- [ ] `docs/lessons/2026-07-12-zhihu-zsxq-fact-rendering.md`：实施中创建并持续记录。
- [ ] 本 issue：合并时 `status: closed`；本 plan：squash 后 `status: done`。

## 后续项

- 对象存储 staging / 不可变 key 与 orphan 回收。
- 是否把 AI/检测/embedding 的输入从 Markdown 改为独立纯文本投影（连生成都禁掉）。
- `raw` → typed `content` 事实列（若上游 API 漂移变痛，YAGNI 备选，两源共同）。
- 实施中发现的非本方案必要改动另开 issue，不扩张本次重写。
