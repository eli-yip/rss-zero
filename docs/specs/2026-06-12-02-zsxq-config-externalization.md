# SPEC Draft：知识星球（zsxq）配置即代码收敛

> 状态：**Draft / 待讨论**
> 范围：`pkg/routers/zsxq/parse/**`、`pkg/routers/zsxq/request/**`、`pkg/routers/zsxq/refmt/**`、`config/**`
> 目标：把散落在解析/请求/重排链路里的硬编码"规则与魔法值"按性质分类处理——运营规则外置到配置，协议常量/一次性边界提为带注释的具名常量——使"改一条运营规则"不再等于"改代码 + 发布"。
> 前置：源自 [2026-06-12-01-zsxq-maintainability-refactor.md](2026-06-12-01-zsxq-maintainability-refactor.md) 附录"配置即代码（后续项）"。本文为讨论稿，方案为方向而非定稿。

---

## 背景与核心判断

附录把四处硬编码笼统归为"外置到配置或至少提为带注释的具名常量"。但这四处**性质不同，处理方式不应一刀切**：

| 项 | 位置 | 性质 | 变化驱动 | 建议处理 |
|---|---|---|---|---|
| 1. 作者过滤 `庄太云` | parse/talk.go:29 | **运营/业务规则** | 业务决定（屏蔽某作者） | **外置到配置** |
| 2. topicIDSkip 黑名单 | parse/topic.go:33 | parser 崩溃 workaround | 发现新的崩溃 topic（低频） | **包级 var；是否进配置待议** |
| 3. 业务码 401/1059/1050 + Sleep(60s) | request.go:201–214 | **协议常量** | zsxq 改 API（极罕见） | **具名常量**（进配置是反模式） |
| 4. 魔法日期 2021-01-01 | refmt/refmt.go:78 | 一次性 backfill 下界 | 基本不变 | **具名常量** |

判断准则：**会因运营/业务决定而变的 → 配置；只因外部协议或历史事实而变的 → 具名常量**。把协议码丢进 TOML 反而让运维有机会改坏不该碰的东西。

---

## 现状证据

### 项 1：作者过滤（运营规则，最高价值）
`parse/talk.go:29`：
```go
if authorID == 184544455455452 || authorName == `庄太云` {
    logger.Info("Skip crawling topic, as it's from zhuangtaiyun", ...)
    return 0, "", ErrNoText
}
```
- 单一作者、ID 与名字"或"匹配，命中即整条 talk 当作无文本跳过。
- 这是典型的"运营要屏蔽某人"规则，改它目前要改代码 + 发版。

### 项 2：topicIDSkip 黑名单
`parse/topic.go:33`：10 个 topicID 的 `[]int{...}` **定义在 `ParseTopic` 函数体内，每次调用重建**；用 `slices.Contains` 线性查找。注释说明多数是"会让 markdown 转换器超时/报错"的 topic。
- 性质是 parser bug 的规避清单，不是运营规则；变化低频。
- 当前实现有微劣化：局部 slice 每次重建、O(n) 查找。

### 项 3：业务码 + Sleep（项一后现状）
`request.go` 的 `Limit.validate` switch（L201–214）：裸数字 `401` / `1059` / `1050` 与 `time.Sleep(60 * time.Second)`。
- 这些是 zsxq API 协议语义，已有类型 `apiResp`/`badAPIResp`/`dataSystemResp` 承载，但分支判断仍用裸字面量。

### 项 4：魔法日期
`refmt/refmt.go:78`：`time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)` 作为 reformat 回溯下界，裸写在循环里。

### 配置载体
`config/toml.go`：全局 `var C TomlConfig`，TOML 反序列化的扁平结构（已有 `Minio`/`Openai`/`Database`/`TestURL.Zsxq` 等块）。新增一个 `[zsxq]` 段是自然的扩展点。

---

## 方案（待定）

### 项 1：作者屏蔽外置到配置（建议优先做）
在 `TomlConfig` 增加 zsxq 段：
```toml
[zsxq]
  blocked_author_ids   = [184544455455452]
  blocked_author_names = ["庄太云"]
```
- parse 层从 `config.C.Zsxq` 读取，命中任一即跳过（保持现有"或"语义与 `ErrNoText` 返回）。
- **待决**：屏蔽是全局还是按 group？当前是全局单作者，倾向先做全局列表，按 group 留作后续。

### 项 2：topicIDSkip
- **最小**：提为包级 `var topicIDSkip = map[int]struct{}{...}`（顺带把 O(n) `slices.Contains` 换成 map 查找），加注释说明用途。
- **可选**：若希望"加一个崩溃 topic 不发版"，再外置到 `[zsxq] skip_topic_ids = [...]`。
- **待决**：这类崩溃 ID 多久新增一次？低频则具名 var 足够；若运营常加则进配置。

### 项 3：业务码 + Sleep → 具名常量
在 request 包定义：
```go
const (
    codeInvalidCookie   = 401  // invalid cookie
    codeTooManyRequests = 1059 // too many requests due to no sign
    codeDataSystemBusy  = 1050 // data system upgrading
)
const noSignBackoff = 60 * time.Second
```
- `Limit.validate` 的 switch 改用常量；`Sleep` 用 `noSignBackoff`。
- **不进配置**：协议码不该让运维改。

### 项 4：魔法日期 → 具名常量
- refmt 包定义 `var reformatFloorTime = time.Date(2021, 1, 1, ...)`（或具名常量 + 注释说明为何是 2021-01-01）。
- **待决**：确认 refmt 是否仍在使用；若是历史一次性工具，可能连这步都不必做（仅加注释）。

---

## 验收（草拟）

- 项 1：屏蔽作者改为读配置；删除 talk.go 中的硬编码 ID/名字；行为不变（命中仍跳过）。
- 项 2：topicIDSkip 不再函数内重建；查找 O(1)；用途有注释。
- 项 3：request 包无裸业务码字面量；switch 走具名常量；行为不变。
- 项 4：refmt 无裸魔法日期（或至少有注释说明）。
- 全程 `go build` / `go vet` 通过；解析行为不变（屏蔽/跳过结果一致）。

---

## 风险/待决汇总

1. 作者屏蔽：全局 vs 按 group？（建议先全局）
2. topicIDSkip：是否值得进配置，取决于新增频率（需你提供运营经验）。
3. refmt：是否仍在用？若废弃则项 4 降级为"加注释"或不做。
4. 配置缺省值：新增 `[zsxq]` 段在旧 config.toml 缺失时应安全降级为空列表（不 panic、不改变现网行为）。

---

## 实施顺序建议

1. **项 1（作者屏蔽外置）** —— 唯一真正的运营规则，价值最高。
2. **项 3（业务码具名常量）** —— 纯 in-code，零风险。
3. **项 2（topicIDSkip 包级 var）** —— 顺带修 O(n)。
4. **项 4（魔法日期）** —— 视 refmt 是否在用，可能仅加注释。

可合并为一个 PR（量都不大），或项 1 单独成 PR、其余打包。
