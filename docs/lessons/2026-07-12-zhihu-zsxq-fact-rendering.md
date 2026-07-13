---
title: "知乎与知识星球事实入库与读时 Markdown 渲染重写复盘"
plan: docs/plans/2026-07-12-zhihu-zsxq-fact-rendering.md
status: draft
areas: [zhihu, zsxq, rss, migrate, db, render]
updated: "2026-07-13"
---

# 知乎与知识星球事实入库与读时 Markdown 渲染重写复盘

承 tombkeeper 已验证的「只存事实、读取期纯渲染」骨架，把知乎与星球两源的 seam 从「抓取期把已排版
Markdown 冻进 `text`」反转为「读取期由纯 `RenderMarkdown` 渲染」。实施经由多个 sub-agent 分阶段落地、
每步验证退出码与测试后再推进。以下记非显然的约束、返工点与判断依据。

## 过程记录

- **分级 pilot 有效**：一份 plan、内部两阶段。ZSXQ 事实完整、无 question-FK、无付费，blast radius 最小，
  先在它身上把 `ContentLoader` / `ContentSnapshot` / 纯渲染 / 事务保存 / 原地删列整套模式跑通；Zhihu
  再按同一骨架 delta 复制，只补它专属的边角（3 类型、付费读取期推导、图片换链、pin 六块与一层 origin
  引用、question-FK 标题降级）。先证模式再铺开，比两源齐头并进风险小得多。

- **原地删列而非 tombkeeper 的 truncate + 重抓**：tombkeeper 必须 truncate 是因其 flight `raw` 不可
  独立重解析；知乎/星球的 `raw` 可重解析，且付费/已删除内容**不保证可重抓**，truncate 会永久丢掉不可
  再生数据。删列前用只读 SQL 审计「`raw` content 为空但 `text` 非空」的行数——zhihu 得 0（article
  0/1794、pin 0/8321、answer 18/24593，且那 18 行 `text` 本就只有一个 `\n`），zsxq 事实在 `raw` + 侧表
  同样完整。即「content 空 ⟺ text 本就空」，证零丢失、零行为变化后，才敢原地删列、不 truncate、不回填。

- **transient 派生渲染保派生值不漂移**：抓取期停止**持久化** Markdown 是硬约束，但 `word_count`、
  内容检测 `detect_status`、embedding、两源 AI 标题这些派生事实当前都从已排版正文算。做法是抓取期用
  内存装配的 `ContentSnapshot` 调**读取期同一个纯 `RenderMarkdown`** 一次、拿临时 Markdown 喂这些派生
  事实、**不落库**——既保证派生值与今天逐字节一致，又没把写侧 renderer 拉回来。其中 `word_count` 的口径
  从「pre-FormatStr」变成「post-FormatStr」，靠 8 个 fixture 的 `md.Count` 不变性证明等价并加回归测试；
  原有的 word_count 测试是同义反复（拿被测函数的输出当期望），发现后重写为对渲染产物计数的有效断言。

- **删列前必须堵死每一条读 `text` 的路径**：只要还有一处消费 `text`，删列就会**静默**把那个端点的正文
  变空。除了 feed 主路径，评审与实现中另发现 `zsxq/random/random.go` 与 `internal/controller/archive/
  utils.go` 里 4 个 Topic-builder 仍在读 `text`，逐一改到读取期从快照渲染后才动删列迁移。这类「删字段
  前 grep 全部读点」是删列型迁移的必修步，漏一处就是线上空正文。

- **保存原子化的真实来源是事务本身，不是写序**：把资源/作者/问题/外部文章 + 内容根行放进同一个 GORM
  事务即保证「要么全可见、要么全不可见」；无外键时行的写入顺序不影响可见性。实现中一度有注释把原子性
  保证归因于「根行最后写」，这是**错误的心智模型**（会诱导后人以为调换写序会破坏保证），已改为如实说明
  保证来自事务边界。

- **autocorrect 会改挂测试断言**：`just fix-lint` 链里的 autocorrect 会给测试里 CJK 与数字/`%`/`@`
  相邻的字面量插空格，从而改掉期望输出、让断言必挂。凡是「期望值须与运行时输出逐字节一致」的字面量，
  用仓库既有约定 `// autocorrect-disable` / `-enable` 就地豁免（散文文档不豁免、正常 fix）。

- **编译诊断以 `go build`/`go vet` 退出码为准**：agent 大改期间语言服务器的编译错误诊断频繁**假阳性**
  ——报的是尚未落盘/半改状态的陈旧中间态，并非真实错误。判断「是否编译通过」一律跑 `go build ./...` /
  `go vet` 看真实退出码，不信诊断快照，否则会为幻影错误做无谓返工。

- **三轴评审各有独立价值**：合并前跑 Standards / Spec / 可读性三个并行 agent（评审者 ≠ 作者）。可读性
  agent 上下文严格受限（**只给完整 diff、核心类型、每个重点函数一个 caller/callee 与关键测试，不给
  issue/plan/作者解释**），拿不到设计意图作拐杖，专抓「要靠猜才懂」的注释与死代码；Spec 抓出 word_count
  口径漂移的同义反复测试；Standards 抓出死常量与重复的 `urlToID`。三者命中的问题互不重叠，印证「读起来
  要靠猜的代码就是要改的代码」——需猜之处对应的是改代码（改名/拆函数/补一行意图），而非补外部文档掩盖。
