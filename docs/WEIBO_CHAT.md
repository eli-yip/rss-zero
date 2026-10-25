# 微博群聊存档核心

`internal/weibochat` 保存群聊消息、用户资料，并转存头像、图片、视频和自定义表情包。当前提供 Go 调用入口和服务端定时抓取，尚无 HTTP API、界面或 CLI。

## 群配置

在 `config.toml` 中列出同一微博账号需要存档的群，群 ID 使用字符串：

```toml
[weibo]
group_ids = [] # 填入实际群 ID，例如 ["群 ID 1", "群 ID 2"]
```

省略此节或使用空数组时不注册微博任务。服务启动时读取 `config.C.Weibo.GroupIDs`；修改列表后重启生效，移除群不会删除已有存档。`settings.debug = true` 时不注册静态抓取任务。

## 定时抓取

`weibo_chat_sync` 每逢 00、15、30、45 分触发，再随机等待 0～5 分钟。延迟和整轮抓取都在调度器 singleton 范围内：上一轮未结束时跳过本次触发，不并发抓取。任务按配置顺序处理各群，共用现有数据库、SUB 和 OSS。

新群首次抓取最新一页（最多 20 条）建立起点；后续从最新消息增量读取到旧水位，不自动回补历史。每群每轮最多读取 100 页、转存 100 个附件，并同步观察到的用户资料和头像。未完成或失败附件留到下轮重试。

达到 100 页仍未接上旧水位时，本轮报错且保留旧水位，已经保存的消息不丢失；需要通过 Go 调用入口以更大的页数上限执行 `Sync` 恢复。单个群失败不阻止后续群；SUB 缺失、过期或明确认证失效时停止整轮，凭据适配器负责通知。

## 调用

复用现有 GORM 数据库、Cookie 服务和 MinIO 文件服务；同一账号可以存档多个群。

```go
store := weibochat.NewPostgresStore(db)
client := weibochat.NewClientWithCookies(nil, cookieService, notifier, logger)
// fileService 的动态类型为现有 *file.FileServiceMinio。
objects, ok := fileService.(weibochat.ObjectStore)
if !ok {
    return errors.New("file service does not support context-aware streaming")
}
users := weibochat.NewUserArchiver(client, store, objects)
messages := weibochat.NewArchiver(client, store, users)

// 只调用一次：从最新一页（最多 20 条）建立存档起点。
initial, err := messages.Initialize(ctx, groupID)
if err != nil {
    return err
}
_ = initial

// 后续每次同步：最多读取 100 页，必须接上旧水位才算成功。
synced, err := messages.Sync(ctx, groupID, 100)
if err != nil {
    return err
}
_ = synced

// 可选：向前回补最多 100 页。Exhausted=false 时可再次调用继续回补。
history, err := messages.Backfill(ctx, groupID, 100)
if err != nil {
    return err
}
_ = history

assets := weibochat.NewAssetArchiver(client, store, objects)
archived, err := assets.ArchiveAssets(ctx, groupID, 100)
if err != nil {
    return err
}
_ = archived
```

同一群的所有调用由调用方串行执行，包括来自不同进程的调用。服务端现有 `MigrateDB` 会通过 AutoMigrate 建立四张表；独立调用者先执行 `db.AutoMigrate(&weibochat.Message{}, &weibochat.SyncState{}, &weibochat.Asset{}, &weibochat.User{})` 并检查错误。

## SUB 凭据

在已登录的微博页面使用现有 `cookie-updater` 导入 Cookie。服务端仅注册 `.weibo.com` 域的 `SUB`，类型为 `CookieTypeWeiboSUB`，其他微博 Cookie 暂不保存。无需修改插件或数据库结构。

沿用现有期限规则：浏览器期限扣除 24 小时安全余量后存入 `ExpireAt`；没有期限时使用现有默认 TTL。`NewClientWithCookies` 每次请求读取当前 SUB，缺失或过期时通知并返回错误，不发送请求。

消息接口明确返回 `21301` 时，客户端使用本次请求携带的 SUB 调用 `InvalidateIfCurrent`，按现有模式软删除并通知，同时返回 `APIError`，本轮存档停止。若 SUB 已被更新，则保留新值。`10012`、未知 API 错误、普通 HTTP 错误、限流和媒体下载错误不触发失效。

独立调用者仍可使用 `NewClient(nil, loadSUB)`，其中 `loadSUB` 返回纯 SUB 值；这个入口不管理 Cookie 存储或失效通知。

## 结果含义

- `Initialize` 只建立最近一页的覆盖范围，已初始化的群返回 `ErrAlreadyInitialized`。
- `Sync` 的 `Complete=true` 表示接上旧水位。失败后已写入的消息保留，水位不变；下次从最新页重跑即可。`MessagesSaved` 是本次新插入的消息数量。
- `Backfill` 每页保存进度，达到页数限制可正常暂停。`Exhausted=true` 仅表示接口返回空页，不证明微博提供了全部历史。
- `ArchiveAssets` 独立于消息同步。错误同时返回已处理数量，失败附件下次调用可重试；达到数量限制时，再次调用处理余下附件。
- 抓取每页消息时同步用户资料，包括已存消息的发送人。头像源地址变化后转存新头像，保留原链接；下载失败保留旧头像配对并返回错误，本页已入库消息保留，但水位不推进，下次调用重试。
- 昵称采用最近观察到的资料，头像采用最近成功转存的版本。头像按资源地址变化判断更新；新浪图片地址仅忽略会轮换的 `Expires`、`ssig` 签名参数，其余路径和参数仍参与判断。签名更新时只保存最新完整原链接，不重复转存。历史消息通过 `sender_id` 关联用户表读取昵称和转存头像，不从消息 JSON 读取当前头像。
- 消息仅保存首次观察内容，不跟踪编辑、撤回或反应。`Raw` 移除 `attitude_info`，未知消息类型仍保留。`ParseError` 非空时表示内容解析不完整。

图片不降级到缩略图，视频不额外下载封面，普通表情字典暂不转存。群聊 `fid` 使用 `msget`；自定义表情包和卡片图片使用上游给出的原图地址。单个附件限制为 512 MiB，传输限时 5 分钟。类型检查用于拒绝 HTML/JSON 错误页，不进行完整图片或视频解码。

`msget` 的图片用法参考 [群聊图片地址构造](https://github.com/alloevil/weibo-chat-auto/blob/e7f8f2346d851fcdaff37d5efbafd5102a0e1647/lib/normalize-message.js#L50-L54) 和 [下载实现](https://github.com/alloevil/weibo-chat-auto/blob/e7f8f2346d851fcdaff37d5efbafd5102a0e1647/scripts/viewer-server.js#L1559-L1570)。消息、附件和头像流程已完成小范围真实群聊和 OSS 联机验证。未与上传原文件比较分辨率或字节内容。
