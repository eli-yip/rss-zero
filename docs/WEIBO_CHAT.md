# 微博群聊存档核心

`internal/weibochat` 保存群聊消息并转存图片、视频和自定义表情包。当前提供 Go 调用入口，尚无 HTTP API、定时任务、界面或 CLI。

## 调用

复用现有 GORM 数据库和 MinIO 文件服务。`loadSUB` 返回当前账号的 SUB 值，不包含 `SUB=`；同一账号可以存档多个群。

```go
store := weibochat.NewPostgresStore(db)
client := weibochat.NewClient(nil, loadSUB)
messages := weibochat.NewArchiver(client, store)

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

// fileService 的动态类型为现有 *file.FileServiceMinio。
objects, ok := fileService.(weibochat.ObjectStore)
if !ok {
    return errors.New("file service does not support context-aware streaming")
}
assets := weibochat.NewAssetArchiver(client, store, objects)
archived, err := assets.ArchiveAssets(ctx, groupID, 100)
if err != nil {
    return err
}
_ = archived
```

同一群的所有调用由调用方串行执行，包括来自不同进程的调用。服务端现有 `MigrateDB` 会通过 AutoMigrate 建立三张表；独立调用者先执行 `db.AutoMigrate(&weibochat.Message{}, &weibochat.SyncState{}, &weibochat.Asset{})` 并检查错误。

## 结果含义

- `Initialize` 只建立最近一页的覆盖范围，已初始化的群返回 `ErrAlreadyInitialized`。
- `Sync` 的 `Complete=true` 表示接上旧水位。失败后已写入的消息保留，水位不变；下次从最新页重跑即可。`MessagesSaved` 是本次新插入的消息数量。
- `Backfill` 每页保存进度，达到页数限制可正常暂停。`Exhausted=true` 仅表示接口返回空页，不证明微博提供了全部历史。
- `ArchiveAssets` 独立于消息同步。错误同时返回已处理数量，失败附件下次调用可重试；达到数量限制时，再次调用处理余下附件。
- 消息仅保存首次观察内容，不跟踪编辑、撤回或反应。`Raw` 移除 `attitude_info`，未知消息类型仍保留。`ParseError` 非空时表示内容解析不完整。

图片不降级到缩略图，视频不额外下载封面，头像和普通表情字典暂不转存。群聊 `fid` 使用 `msget`；自定义表情包和卡片图片使用上游给出的原图地址。单个附件限制为 512 MiB，传输限时 5 分钟。类型检查用于拒绝 HTML/JSON 错误页，不进行完整图片或视频解码。

`msget` 的图片用法参考 [群聊图片地址构造](https://github.com/alloevil/weibo-chat-auto/blob/e7f8f2346d851fcdaff37d5efbafd5102a0e1647/lib/normalize-message.js#L50-L54) 和 [下载实现](https://github.com/alloevil/weibo-chat-auto/blob/e7f8f2346d851fcdaff37d5efbafd5102a0e1647/scripts/viewer-server.js#L1559-L1570)。本项目已用模拟 HTTP、文件服务及独立 Postgres 验证流程，尚未用真实群聊和 OSS 联机验证，也未与上传原文件比较分辨率或字节内容。
