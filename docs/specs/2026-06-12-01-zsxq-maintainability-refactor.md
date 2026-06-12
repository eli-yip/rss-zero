# SPEC：知识星球（zsxq）模块可维护性重构

> 状态：**已定稿 / 待实现**
> 范围：`pkg/routers/zsxq/**`（request / crawl / render 三处）
> 目标：在不改变对外行为的前提下，消除三处对可维护性影响最大的重复/耦合，降低后续改动的成本与出错率。
> 讨论记录：三项"待决"点已逐一用代码事实与线上数据确认，方案已定稿。下文每项的"方案"为最终实现方向。

---

## 背景

zsxq 模块经过多轮迭代，核心爬取/渲染链路出现了大量复制粘贴与配置即代码的痕迹。三个独立 review pass + 人工核对一致指向以下问题：改一条逻辑要在多处同步修改，且这些副本已经各自发生了细微劣化（行为不一致、nil 校验不统一、死代码）。本 SPEC 提出三项收敛重构。

非目标（本轮不做）：
- 不改对外 RSS / 导出 / 归档的可观测行为
- 不引入新的存储或第三方依赖
- 不重写 cron 调度与 controller 层 channel 编排（仅记录为后续项）

---

## 重构项一：请求重试循环收敛（优先级中）

### 现状
`pkg/routers/zsxq/request/request.go` 中 `Limit` / `LimitRaw` / `LimitStream` / `NoLimit` 四个方法（L114–319）结构 80–90% 雷同：
生成 taskID → 日志 → `for i < maxRetry` → `setReq` → `client.Do` → 查状态码 → 读 body → 失败 `continue`。

真正差异仅三轴：
1. 是否经过 `limiter`（`NoLimit` 不经过）
2. 200 之后如何处理响应（返回 `[]byte` 还是移交 `*http.Response`）
3. 是否校验业务 JSON（仅 `Limit` 解析 `succeeded` / `code`，并对 401/1059/1050 做分支处理）

> 补充决定：`NoLimit` 全项目无任何调用者（仅接口声明），本项一并删除。删除后 `Limit`/`LimitRaw`/`LimitStream` 三者**都走 limiter**，故核心方法不再需要 `useLimiter` 参数（轴 1 消失）。

### 问题
- 任一重试改动（backoff、`ctx` 超时、日志字段）需改四处，易漏改。
- 副本已劣化：四个方法的 body close 行为各不相同——`Limit`/`LimitRaw`/`NoLimit` 在**循环内 `defer resp.Body.Close()`**（L135/216/300），重试 5 次会累积 5 个未关闭 body 到函数返回才释放，是**潜在泄漏**；`LimitStream` 仅错误分支 close（L264），成功路径移交调用方。
- 业务码（401/1059/1050、`time.Sleep(60s)`）与传输重试糊在同一循环里，分层不清。

### 方案（已定稿）
核心循环抽为一个私有方法，**返回 `*http.Response`（保证 200、body 未读）**，把"200 之后做什么"交给 `validate` 回调：

```go
func (r *RequestService) doWithRetry(
    ctx context.Context, u string,
    validate func(resp *http.Response) (done bool, err error),
    logger *zap.Logger,
) (*http.Response, error)
```

`validate` 语义：
- `done=true` → 核心返回此 `resp`；
- `done=false` → 重试（body 由 `validate` 读完即 close）；
- `err != nil` → 致命，立即返回（供 401 → `ErrInvalidCookie`）。

三个公开方法（删 `NoLimit` 后）降为薄包装，**返回类型分叉通过闭包捕获溶解**，核心永远只返回 `*http.Response`：
- `Limit`：`validate` 读 body、解析 `succeeded`；401→致命、1059→`Sleep(60s)`+重试、1050 及其它→重试；成功时把 body 存入闭包变量。业务码全部收敛于此，核心保持纯传输。
- `LimitStream`：`validate` 不碰 body，直接 `return true, nil`，核心返回 resp、body 存活。
- `LimitRaw`：`validate` 读完即 close，闭包带出 body。

为什么不用"三态 action 枚举"：枚举只是挪了 switch，没解决**返回类型分叉**与 **body 生命周期**两个根本问题。`(done, err)` 回调 + 闭包捕获同时解决两者，且无需泛型。

### 验收
- 保留的三个方法对外签名与行为不变（含错误类型 `ErrInvalidCookie` / `ErrMaxRetry`）；`NoLimit` 从接口与实现中删除。
- 重试策略只有一处真相；约 200 行降至核心 ~35 行 + 三个 5–10 行包装。
- body close 行为在所有分支统一且无泄漏：循环内 `defer` 泄漏一并修掉（读字节的分支在 `validate` 内即时 close，stream 分支故意不 close）。
- 业务码（401/1059/1050、`Sleep`）位于 `Limit` 的 `validate`，传输层不再感知 zsxq 业务语义。

---

## 重构项二：删除死代码回溯分支（优先级最高，先做）

### 现状
`pkg/routers/zsxq/crawl/group.go` 的 `CrawlGroup` 含两个 `for !finished` 循环：正向（L36–90）、回溯（L99–144），两段 40+ 行几乎逐行相同。

### 确认结论（已用代码核实）
- `CrawlGroup` **全项目仅 1 个调用点** `pkg/routers/zsxq/cron/crawl.go:277`，传参恒为 `backtrack=false, earliestTopicTimeInDB=time.Time{}`。
- 回溯循环（L92–144）**永不执行，是死代码**。
- `GetEarliestTopicTime`（`db/topic.go:31`+68）**无任何调用者**，是回溯分支唯一会用到的 DB 入口。
- 作者已确认历史上无运维手动触发回溯的用法。

### 方案（已定稿）
直接删除，不做"抽 `fetchAndParsePage`"——删完只剩一个正向循环，本无重复可去：
1. 删除回溯循环（group.go L92–144）。
2. 删除 `CrawlGroup` 的 `backtrack bool` 与 `earliestTopicTimeInDB time.Time` 两个参数，更新唯一调用点。
3. 删除 `GetEarliestTopicTime`（接口声明 + 实现），确认无其它引用后移除。

### 验收
- 正向爬取行为不变。
- 无人调用的参数/分支/方法全部移除，`go build` / `go vet` 通过。

---

## 重构项三：object → URI 构造收敛（需线上数据验证，已完成）

### 现状
`pkg/routers/zsxq/render/markdown.go` 中 `generateFilePartText`、`generateImagePartText`、`renderQA` 等 5+ 处反复出现（L157–262）：

```go
object, err := m.db.GetObjectInfo(id)
uri := fmt.Sprintf("%s/%s", object.StorageProvider[0], object.ObjectKey)
```

### 问题（已核实）
- `StorageProvider[0]` 的"永远取第 0 个"假设散布各处，无越界保护。
- nil 校验不一致：`generateImagePartText` 写了两遍冗余判断（L191 复合条件 + L194 重复），且 L191 在 `err==nil` 时 wrap 一个 nil。
- **静默吞错 bug**：`renderQA`（L218/234/257）`if err != nil || object.StorageProvider == nil { return err }`——当 provider 为 nil 但 err 为 nil 时 `return nil`，**漏渲染整段图片/语音且不报错**，数据问题被掩盖。
- 转义不一致：file 用 `url.PathEscape(ObjectKey)`（L174），image/voice 完全不转义（L199/221/237/260）。

### 线上数据验证（linkerlab-us-2 / rss-db）
`zsxq_object` 表：image 509、file 45、voice 39 行。

- image / voice 的 `object_key` 全是 `zsxq/<数字>.<ext>`，**零特殊字符**。
- file 的 45 行**全部**含中文 + 全角 `：`，当前走 `PathEscape`。
- 关键：`url.PathEscape` 会把 `/` 也编码为 `%2F`，故当前 file 产物形如 `provider/zsxq%2F…%EF%BC%9A….docx`（连路径分隔符都编码）。
- 用真实 OSS（`https://oss.darkeli.com/rss`）实测同一 file：**整串转义（`%2F`）与按段转义（保留 `/`）两种 URL 都返回 200**。

### 方案（已定稿）
给 `*Object` 增加 `URI()` 方法（放在 `db/object.go`，与字段同处最自然），统一为**按 `/` 分段、段内 `PathEscape`、分隔符保留**：

```go
var ErrNoStorageProvider = errors.New("object has no storage provider")

func (o *Object) URI() (string, error) {
    if len(o.StorageProvider) == 0 {
        return "", fmt.Errorf("%w: object_key=%s", ErrNoStorageProvider, o.ObjectKey)
    }
    segs := strings.Split(o.ObjectKey, "/")
    for i, s := range segs {
        segs[i] = url.PathEscape(s)
    }
    return o.StorageProvider[0] + "/" + strings.Join(segs, "/"), nil
}
```

所有调用点收敛为 `uri, err := object.URI(); if err != nil { return ..., err }`。

为什么"按段"而非"整串 PathEscape"：

| type | 当前产物 | 新产物（按段） | 回归 |
|---|---|---|---|
| image (509) | `provider/zsxq/123.jpg`（不转义） | 字节完全相同 | 零 |
| voice (39) | 同上 | 字节完全相同 | 零 |
| file (45) | `provider/zsxq%2F…%EF%BC%9A….docx` | `provider/zsxq/…%EF%BC%9A….docx` | 字节变，但两种实测均 200 |

按段转义是唯一让 image/voice 字节不变、同时给 file 更干净且已验证可用链接的方案，且语义正确（路径分隔符保持分隔符，仅文件名段转义）。

### 验收
- 所有 object URI 走同一 `URI()` 方法；空 provider 返回明确错误，无越界 panic。
- `renderQA` 的静默吞错修复：nil-provider 不再被当成"无图"跳过，而是返回错误。
- image / voice 渲染产物逐字节不变；file 链接 `%2F`→`/`，已实测可用。

---

## 附录：配置即代码（记录为后续项，本轮不做）

散落多处、量级稍小但值得跟进：
- `parse/topic.go` 硬编码 `topicIDSkip` 跳过列表
- `parse/talk.go:29` 业务过滤规则 `authorID == 184544455455452 || authorName == "庄太云"` 写进解析器
- `request.go` 裸错误码 `401 / 1059 / 1050` 与 `time.Sleep(60s)`
- `refmt.go` 魔法日期 `2021-01-01`

方向：外置到配置或至少提为带注释的具名常量，使"改一条运营规则"不再等于"改代码 + 发布"。

---

## 实施顺序

1. **项二（删死代码）** —— 收益立等、风险最低，先做。
2. **项一（请求重试收敛）** —— 接口边界清晰，风险可控。
3. **项三（URI 收敛）** —— 线上数据已验证零回归，可安全实施。

每项独立成 PR，互不阻塞。
