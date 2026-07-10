# PLAN：项三 — object → URI 构造收敛

> 对应 SPEC：[2026-06-12-01-zsxq-maintainability-refactor.md](../issues/2026-06-12-zsxq-maintainability-refactor.md) 重构项三
> 分支：`feat-zsxq-object-uri`
> 风险：低（线上数据已验证零回归；image/voice 字节不变，file 链接两形态实测均 200）

## 目标

把散落在 markdown.go 的 5 处 `provider[0] + "/" + ObjectKey` 收敛为 `Object.URI()`，统一转义（按 `/` 分段、段内 PathEscape）、补越界/nil 防护，并修掉 renderQA 的静默吞错与 generateImagePartText 的重复 nil 判断。

## 前置事实（已核实）

- 5 处构造点全在 `pkg/routers/zsxq/render/markdown.go`：
  1. L165–174 `generateFilePartText`（file，当前 `url.PathEscape(ObjectKey)`）
  2. L190–199 `generateImagePartText`（image，不转义；L191+L194 重复 nil 判断）
  3. L217–221 `renderQA` 提问图片（不转义；L218 `err!=nil || provider==nil` → `return err`，**provider nil 但 err nil 时静默 return nil**）
  4. L233–237 `renderQA` 语音（同上静默吞错）
  5. L256–260 `renderQA` 回答图片（同上静默吞错）
- 写入侧 `StorageProvider` 恒为单元素（parse/image.go:58、talk.go:81、q&a.go:83 均 `[]string{s.file.AssetsDomain()}`），`[0]` 实践安全但无防护。
- 线上 `zsxq_object`：image 509 / voice 39 全是 `zsxq/<数字>.<ext>`（零特殊字符）；file 45 全含中文 + 全角 `：`。按段转义后 image/voice 字节不变，file `%2F`→`/`（两形态实测 200）。

## 实现设计（来自 SPEC 定稿）

`pkg/routers/zsxq/db/object.go` 增加：

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

新增 import：`errors`、`fmt`、`net/url`、`strings`。

## 步骤

### 1. 建分支

```
git checkout -b feat-zsxq-object-uri
```

### 2. 加 `Object.URI()` + 单测

- 在 db/object.go 实现上述方法。
- 新增 `pkg/routers/zsxq/db/object_test.go`（白盒），用例：
  - image-like：`provider=["https://oss.x/rss"]`, key=`zsxq/123.jpg` → `https://oss.x/rss/zsxq/123.jpg`（分隔符保留、无转义）。
  - file-like：key=`zsxq/118-东昌：x.docx` → 段内转义、`/` 保留（断言含 `/zsxq/`，且文件名段被 PathEscape）。
  - 空 provider（nil 与 `[]string{}`）→ `errors.Is(err, ErrNoStorageProvider)`。
- 先跑通单测。

### 3. 改 markdown.go 5 处调用点

统一为：

```go
object, err := m.db.GetObjectInfo(id)
if err != nil {
    return ..., fmt.Errorf("failed to get object %d info: %w", id, err)
}
uri, err := object.URI()
if err != nil {
    return ..., fmt.Errorf("failed to build object %d uri: %w", id, err)
}
```

- generateFilePartText / generateImagePartText：返回 `("", err)`；去掉 image 的重复 nil 判断。
- renderQA 三处：返回 `err`（**修静默吞错**：原 `if err!=nil || provider==nil { return err }` 在 provider nil 时返回 nil）。
- 删除 markdown.go 中变为未使用的 `net/url` import（PathEscape 已移入 URI()，需确认无其它 url 用法）。

### 4. 校验

```
go build ./...
go vet ./pkg/routers/zsxq/...
go test ./pkg/routers/zsxq/db/... ./pkg/routers/zsxq/render/...
```

- 若 render 包有渲染快照/golden 测试，确认 image/voice 输出字节不变。

### 5. 线上产物复核（可选）

- image/voice 字节不变（设计保证），file 链接形态变更已在 SPEC 阶段实测 200，无需重复。

## 验收

- 5 处 URI 构造全部走 `Object.URI()`；空 provider 返回 `ErrNoStorageProvider`，无越界 panic。
- renderQA 不再静默吞错：nil-provider 返回错误而非当作"无图"跳过。
- generateImagePartText 重复 nil 判断消除。
- image/voice 渲染产物逐字节不变；file 链接 `%2F`→`/`（已实测可用）。
- 新增 object_test.go 通过；`go build`/`go vet` 通过。

## 提交与合并

- Conventional Commit（英文），如：
  `refactor(zsxq): add Object.URI and converge render URI construction`
- 完成后请作者 review，批准后 squash merge 进 `master` 并删分支。
- 更新 `docs/PROGRESS.md`；本项完成后整个 SPEC 三项收尾。
