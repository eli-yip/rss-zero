# PLAN：zsxq 配置即代码收敛（四项合一）

> 对应 SPEC：[2026-06-12-02-zsxq-config-externalization.md](../issues/2026-06-12-zsxq-config-externalization.md)
> 分支：`feat-zsxq-config`
> 风险：项 1 中（配置 + 现网同步，漏配会让 `庄太云` 屏蔽回归）；项 2/3/4 低（纯 in-code 重命名）

## 目标

一个 PLAN/分支内完成四项：作者屏蔽外置到全局配置（项 1），topicIDSkip 提包级 map（项 2），业务码/backoff 提具名常量（项 3），refmt 魔法日期提具名常量（项 4）。

## 前置事实（已核实）

- `config.toml`（repo 根）**被 gitignore**，是本地密钥文件；`deploy/config.toml` **被跟踪**，是模板。现网在 `linkerlab-us-2:~/services/rss-zero/config.toml`。
- go-toml v2 忽略未知键 → **提前给现网 config.toml 加 `[zsxq]` 段是安全的**（旧二进制无视），部署新镜像后才生效。
- `config.C` 是全局 `TomlConfig`，refmt 等已直接读全局；parse 包目前**未** import config，新增 `parse → config` 依赖无环（config 不依赖 zsxq）。
- parse 当前硬编码：talk.go:29 `authorID == 184544455455452 || authorName == "庄太云"`；topic.go:33 函数内 `[]int{...}` + `slices.Contains`（O(n)）。
- request.go（项一后）：`Limit.validate` switch 用裸 `401/1059/1050` 与 `time.Sleep(60 * time.Second)`。
- refmt.go:78：`time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)` 裸写在循环里；refmt 仍在使用。

## 步骤

### 0. 建分支

```
git checkout -b feat-zsxq-config
```

### 项 3：业务码 + backoff → 具名常量（先做，零风险）

`pkg/routers/zsxq/request/request.go`：

```go
const (
    codeInvalidCookie   = 401  // invalid cookie
    codeTooManyRequests = 1059 // too many requests due to no sign
    codeDataSystemBusy  = 1050 // data system upgrading
)
const noSignBackoff = 60 * time.Second
```

- `Limit.validate` 的 `case 401/1059/1050` 改用常量；`time.Sleep(60 * time.Second)` → `time.Sleep(noSignBackoff)`。
- 行为不变；现有 request_test.go 应仍全绿。

### 项 4：refmt 魔法日期 → 具名常量

`pkg/routers/zsxq/refmt/refmt.go`：

```go
// reformatFloorTime is the lower bound for reformatting: topics older than
// this are not reprocessed.
var reformatFloorTime = time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
```

- L78 `lastTime.Before(time.Date(2021,...))` → `lastTime.Before(reformatFloorTime)`；日志文案保留或引用常量。

### 项 2：topicIDSkip → 包级 map var

`pkg/routers/zsxq/parse/topic.go`：

- 把函数内 `topicIDSkip := []int{...}` 提为包级 `var topicIDSkip = map[int]struct{}{ 2855142121821411: {}, ... }`，保留注释（多为 markdown 转换器超时/报错的 topic）。
- `ParseTopic` 内 `slices.Contains(topicIDSkip, id)` → `_, skip := topicIDSkip[topic.TopicID]; if skip { ... }`。
- 若 `slices` 在该文件无其它用处则移除其 import。

### 项 1：作者屏蔽外置到全局配置

1. `config/toml.go`：新增结构与字段
   ```go
   type ZsxqConfig struct {
       BlockedAuthorIDs   []int    `toml:"blocked_author_ids"`
       BlockedAuthorNames []string `toml:"blocked_author_names"`
   }
   // TomlConfig 内：
   Zsxq ZsxqConfig `toml:"zsxq"`
   ```
2. `pkg/routers/zsxq/parse/talk.go`：import config + slices，替换硬编码
   ```go
   if slices.Contains(config.C.Zsxq.BlockedAuthorIDs, authorID) ||
       slices.Contains(config.C.Zsxq.BlockedAuthorNames, authorName) {
       logger.Info("Skip crawling topic, blocked author", zap.Int("topic_id", topic.TopicID),
           zap.Int("author_id", authorID), zap.String("author_name", authorName))
       return 0, "", ErrNoText
   }
   ```
   - 保持现有"或"语义与 `ErrNoText` 返回。缺省（空列表）→ 不屏蔽任何人、不 panic。
3. 配置文件三处补 `[zsxq]` 段：
   - `deploy/config.toml`（模板，提交）：含 `庄太云` 示例值，作为部署参考。
   - 本地 `config.toml`（gitignore，不提交）：补上以保本地行为/测试一致。
   - 现网 `linkerlab-us-2:~/services/rss-zero/config.toml`（ssh 编辑）：
     ```toml
     [zsxq]
       blocked_author_ids   = [184544455455452]
       blocked_author_names = ["庄太云"]
     ```
     **先备份再改**；旧二进制忽略该段，安全。部署新镜像后生效。

### 校验

```
go build ./...
go vet ./pkg/routers/zsxq/... ./config/...
go test ./pkg/routers/zsxq/request/... ./pkg/routers/zsxq/db/...
```

- 可选：为项 1 加一个针对屏蔽判断的小测试（设置 `config.C.Zsxq` 后断言命中/不命中）。

## 验收

- 项 1：talk.go 无硬编码作者 ID/名字；读 `config.C.Zsxq`；空配置不屏蔽且不 panic；现网 config.toml 已补 `庄太云`。
- 项 2：topicIDSkip 为包级 map，O(1) 查找，不在函数内重建。
- 项 3：request 包无裸业务码字面量；switch/Sleep 走具名常量；request_test.go 全绿。
- 项 4：refmt 无裸魔法日期。
- `go build ./...` / `go vet` 通过。

## 提交与合并

- 小步提交，Conventional Commit（英文），建议按项分提交：
  - `refactor(zsxq): name request business codes and backoff`
  - `refactor(zsxq): name refmt reformat floor time`
  - `refactor(zsxq): hoist topicIDSkip to package-level map`
  - `feat(zsxq): externalize blocked authors to config`
- 完成后请作者 review，批准后 squash merge 进 `master` 并删分支。
- 更新 `docs/PROGRESS.md`。
- **部署提醒**：项 1 生效需重新构建并部署镜像（rss-zero-release 流程，单独步骤，本 PLAN 不含部署）；现网 config.toml 已提前就绪。
