# SPEC: 知乎付费内容标识（答案 / 文章）

- 日期：2026-06-20
- 状态：已 review，决策已定，待实现

## 背景

知乎答案和专栏文章存在「付费专栏内容」。由于抓取账号未付费，付费内容的正文在
知乎 API 返回里**本身就是不完整的**（只有免费预览段，结尾常在句子中途截断）。这是
预期行为，我们无法也不打算拿到全文。

现状（连真实库 `rss-db` 核实）：

- **答案**：解析时已有 `answer_type == "paid_column_content"` 的判断，命中后把
  `**该文章为付费专栏内容**` 这行**内联粗体**提示**烘焙进** `zhihu_answer.text`
  存库（解析期 [parse/answer.go](../../pkg/routers/zhihu/parse/answer.go)、重排期
  [refmt/answer.go](../../pkg/routers/zhihu/refmt/answer.go) 各一处，函数
  `AddPaidColumnContentNotice`）。提示无原文链接。
- **文章**：完全没有付费处理。`zhihu_article.raw` 里其实带判定字段，但 article
  api model（[api_models/article.go](../../pkg/routers/zhihu/parse/api_models/article.go)）
  没解析它们。

### raw 字段实测结论（canglimo 及全表）

`zhihu_article.raw`（bytea，知乎 API 原始 JSON）里有两个相关字段：

| 字段           | 付费                         | 免费           |
| -------------- | ---------------------------- | -------------- |
| `article_type` | `"paid_column_content"`      | `"normal"`     |
| `paid_info`    | `{"content": "<正文 HTML>"}` | `{}`（空对象） |

全表分布（佐证）：

```
article_type=normal              + paid_info 空    : 1182  免费
article_type=paid_column_content + paid_info 非空  :  397  付费
article_type=(空)                + paid_info 空    :  167  旧数据无该字段（免费）
article_type=normal              + paid_info 非空  :   21  付费但被知乎标成 normal（边界）
```

**文章判定规则**因此取「`article_type == "paid_column_content"` **或** `paid_info`
非空」，后者用于兜住那 21 篇被错标为 normal 的旧付费文。**答案判定**用
`answer_type == "paid_column_content"`（与现状一致）。

canglimo 最近文章实测全部为付费（`paid_column_content` + `paid_info.content` 非空）。

## 设计取舍：单一数据源（烘焙进 text），不持久化标志、不新增渲染注入点

正文 body 有 **4 个消费点，全部直接读 `answer.Text` / `article.Text`**：

| 消费点       | 位置                                                                                             | 原文链接            |
| ------------ | ------------------------------------------------------------------------------------------------ | ------------------- |
| RSS          | [internal/rss/zhihu.go](../../internal/rss/zhihu.go)（`render.RSS{Link, Text}`）                 | 有                  |
| 合集 export  | [export/export.go](../../pkg/routers/zhihu/export/export.go) + `single.go` → `FullTextRender`    | 有（内部）          |
| Archive 单页 | [controller/archive/zhihu_archive.go](../../internal/controller/archive/zhihu_archive.go) → 同上 | 有                  |
| Archive 列表 | [controller/archive/utils.go](../../internal/controller/archive/utils.go)（`Body: *.Text`）      | 有（`OriginalURL`） |

付费提示当前在**生产端**烘焙进 `.Text`，4 个消费点便「免费」全部带上——这是单一
数据源的好处。若把 `.Text` 改为纯净（不含提示），就得在 ≥3 个消费端分别注入并迁移
strip 存量正文，发散且更险。**本 SPEC 维持烘焙模型，只把烘焙内容升级、并扩到文章**：

- 提示从无链接的粗体 `**该文章为付费专栏内容**` 升级为带链接的**引用块**，置于正文开头：
  `> 本文为付费内容，请点击[原文链接](原文URL)查看全文`
  （解析期 answer/article 的 ID 都在手，链接就地生成；4 个消费点一行不用改）。

**不持久化「是否付费」标志**（review 决策）：是否付费仅在解析/迁移时由 `raw` 现算并
用于决定是否烘焙引用块，**不新增数据库列**。展示完全走 `.Text` 内的引用块。

**前端无需改动**（review 决策）：付费提示已在 `Body`/正文里，前端按 Markdown 渲染即得
引用块，不单独加图标/Tooltip，`/archive` 的 Topic JSON 也不新增字段。

## 方案

### 1. 解析填充 + 提示烘焙

- 抽共享判定函数（供解析与 migration 复用，避免规则漂移）：
  - `IsPaidAnswer(answerType string) bool` → `answerType == "paid_column_content"`
  - `IsPaidArticle(articleType string, paidInfo json.RawMessage) bool`
    ```go
    // NOTE: 付费判定取双条件之并。article_type=="paid_column_content" 是主信号；
    // 但实测全表有 21 篇 paid_info 非空却被知乎标成 article_type=="normal" 的旧付费文，
    // 故再加 paid_info 非空兜底，避免漏判。详见 SPEC 2026-06-20-01。
    // paid_info 判空：len>0 && != "{}" && != "null"。
    ```
- 升级提示函数：把现有 `AddPaidColumnContentNotice` 换成带链接的引用块版本
  （入参为正文与原文链接），命中付费时把引用块拼到正文**之前**。引用块文案：
  `> 本文为付费内容，请点击[原文链接](原文URL)查看全文`。
- **答案**：article api model 已有 `AnswerType`；`ParseAnswer`（[parse/answer.go](../../pkg/routers/zhihu/parse/answer.go)）
  命中付费时用 `GenerateAnswerLink(questionID, answerID)` 烘焙引用块。
  `refmt/answer.go` 同步。不再写入旧粗体提示。
- **文章**：article api model 增加 `ArticleType string \`json:"article_type"\``与`PaidInfo json.RawMessage \`json:"paid_info"\``；`ParseArticle`（[parse/article.go](../../pkg/routers/zhihu/parse/article.go)）命中付费时用`GenerateArticleLink(articleID)`烘焙引用块；`refmt/article.go` 同步（文章此前无任何
  付费处理，本次新增）。

### 2. Migration 回填历史正文

新增 [internal/migrate/20260620.go](../../internal/migrate/20260620.go) `Migrate20260620`，
仿 [20260612.go](../../internal/migrate/20260612.go) 的 `backfillRow` 投影 + 逐行 UPDATE，
**只回填 `text`**（无新列）：

- 遍历 `zhihu_answer`：投影 `id, raw, text`（答案另需 `question_id` 以拼链接）。
  从 `raw` 取 `answer_type`，`IsPaidAnswer` 命中且 `text` 尚无新引用块时，把引用块补进
  `text`：先 strip 旧 `**该文章为付费专栏内容**` 行，再前置新引用块。`UPDATE text`。
- 遍历 `zhihu_article`：投影 `id, raw, text`。从 `raw` 取 `article_type` + `paid_info`，
  `IsPaidArticle` 命中且 `text` 尚无新引用块时前置引用块。`UPDATE text`。
- 幂等：按引用块/链接前缀判存在则跳过；可重复跑。记录 命中/补写/跳过/失败 计数日志。
- 为什么迁移要碰正文：提示是解析期烘焙的，存量内容（尤其 canglimo ~400 篇文章）
  `.Text` 里没有新引用块；不补则历史付费条目在 RSS/export/archive 里看不到提示。
- 注册控制器入口，仿
  [controller/migrate/migrate.go](../../internal/controller/migrate/migrate.go) 既有
  `Migrate20260612`（`go migrate.Migrate20260620(...)`）+ 路由。

## 影响面 / 非目标

- 不尝试获取付费全文（抓取天然不完整，接受现状）。
- 不新增数据库列；不改 RSS 抓取与去重逻辑；不改前端；不改 `/archive` Topic JSON 结构。
- 是否付费不落库，每次按 `raw` 现算——本特性的唯一产物就是 `.Text` 开头的引用块。

## review 决策记录

1. **迁移补写历史正文**：**接受**。migration 为付费历史条目把引用块补进 `.Text`
   （幂等、替换答案旧提示）。这是烘焙模型下让历史条目显示提示的必要代价。
2. **不持久化 `paid` 列**：**接受**。前端不读、展示走 text，故不加列，是否付费现算即可。

## 验收

- 付费条目（答案 + 文章）在 RSS / 合集 export / `/archive`（单页与列表）正文**开头**均为
  带原文链接的引用块；非付费条目无变化。
- 历史付费条目经 migration 后同样带上引用块；migration 可重复跑且不重复插入。
- 答案旧的 `**该文章为付费专栏内容**` 内联提示被新引用块取代，不出现重复。
- canglimo 文章（付费）烘焙引用块，链接为 `https://zhuanlan.zhihu.com/p/<id>`。
