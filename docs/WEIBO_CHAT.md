# 微博群聊存档核心

`internal/weibochat` 保存群聊消息、用户资料，并转存头像、图片、视频和自定义表情包。当前提供消息浏览 API、Go 调用入口和服务端定时抓取，尚无界面或 CLI。

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

消息同步失败（包括初始化、头像转存和水位保存失败）会在本轮结束时汇总为一条 Bark，包含运行 ID、失败群数和至多 3 条错误示例。每条示例最多 500 个字符，正文最多 1000 个字符；完整错误保留在日志中，日志字段 `cron_job_id` 对应通知中的运行 ID。凭据错误沿用现有 Cookie 通知，不重复计入同步摘要；即使凭据错误提前停止本轮，此前的同步失败仍会通知。独立附件转存失败只记日志并留待重试。持续同步失败会每轮通知，Bark 发送失败只记日志。

## 消息浏览 API

```http
GET /api/v1/weibo/groups
GET /api/v1/weibo/groups/5152823595766382/senders
GET /api/v1/weibo/groups/5152823595766382/messages?sender_id=123&asset_kind=image&limit=50
```

接口沿用服务端管理员访问规则，只读取本地存档，不请求微博。

- `groups` 返回配置中的群与已有存档的群，按数值群 ID 升序去重。每项含 `group_id`、`configured`、`initialized_at`、`last_synced_at`；未初始化的时间为 `null`。移除配置不会隐藏已有存档。
- `senders` 返回该群存档中出现过的发件人，按数值 UID 升序排列。每项含 `uid`、最新 `screen_name` 和转存后的 `avatar_url`。这不是当前群成员名单；缺少用户资料时仍返回 UID。
- `messages` 返回数值消息 ID 升序列表，支持以下筛选。合法但尚未存档的群返回空列表。

| 参数                | 含义                                                  |
| ------------------- | ----------------------------------------------------- |
| `q`                 | 正文关键词搜索，空白分词取 AND，双引号包裹连续短语    |
| `highlight_q`       | 只控制高亮，不过滤；缺省使用 `q`，显式空值关闭高亮    |
| `sender_id`         | 可重复，同一参数多个 UID 取 OR                        |
| `sent_from`         | 发送时间起点，包含，RFC3339                           |
| `sent_before`       | 发送时间终点，不包含，RFC3339                         |
| `asset_kind`        | 可重复，支持 `image`、`video`、`sticker`，多个值取 OR |
| `before_message_id` | 排他游标，取更早且最近的消息                          |
| `after_message_id`  | 排他游标，取更新且最近的消息；不能与前者同时传入      |
| `limit`             | 默认 50，范围 1～100                                  |

不同筛选维度取 AND；省略参数表示不限制。时间示例为 `2026-10-25T00:00:00Z`，使用 `+08:00` 时需将 `+` 编码为 `%2B`。起点必须早于终点。参数为空、格式错误或类型不受支持时返回 `400`。

附件筛选选择包含匹配附件的整条消息，返回时保留全部附件；`image` 不包含 `sticker`。普通文件类型尚未结构化解析，不支持按 PDF、ZIP 等筛选。未知消息默认保留；设置时间范围时，缺少发送时间的消息不匹配；设置附件条件时，只返回已确认匹配的消息。

搜索示例（`q` 解码后为 `数据库 "hello world"`）：

```http
GET /api/v1/weibo/groups/5152823595766382/messages?q=%E6%95%B0%E6%8D%AE%E5%BA%93%20%22hello%20world%22
```

`q` 和 `highlight_q` 使用相同语法：引号外空白分词，双引号内空白原样保留；`\"` 表示字面双引号，`\\` 表示字面反斜杠，其他反斜杠原样保留。单引号、中文引号、`%`、`_` 都是普通字符。未闭合双引号、空短语 `""`、重复参数或超过 256 个 Unicode 码点返回 `400`。空字符串或全空白等同于无搜索。英文 ASCII `A–Z` 不区分大小写，其他字符按原文匹配，不做简繁、同义词或拼音转换。

搜索只匹配正文，不匹配用户名称或原始 JSON。每个词都必须出现，顺序不限；`highlight_q` 可指定不同的词，只影响标记。消息新增 `text_matches`：

```json
{"text":"今天讨论数据库","text_matches":[{"start":4,"end":7}]}
```

区间为原文 Unicode 码点的左闭右开范围，不是 UTF-8 字节或 UTF-16 单元。返回全部匹配并合并重叠区间；没有匹配时为 `[]`。前端可通过 `Array.from(text)` 按码点切片，不需要再次实现搜索解析。

响应使用现有 `{ "message": "...", "data": ... }` 包装。群和发件人列表的 `data` 为 `{ "items": [] }`。消息列表的 `data` 为：

```json
{
  "items": [],
  "has_more": false
}
```

不传游标时取最新 N 条；传入 `before_message_id` 或 `after_message_id` 时取该方向最近 N 条。所有结果均按数值 ID 升序返回。`has_more` 表示本次查询方向是否还有匹配消息；不传游标时表示更早方向。

向上加载使用当前窗口最小 ID 作为 `before_message_id`；向下加载使用最大 ID 作为 `after_message_id`，两侧分别维护加载状态。保留搜索和筛选参数；条件变化后清空窗口重新请求。不返回总数。到达底部后，可再次用最大 ID 请求 `after_message_id` 刷新新存档消息。

消息字段包括 `group_id`、`message_id`、`sender_id`、`sender`、`sent_at`、`text`、`links`、`assets`、`message_type`、`media_type`、`sub_type` 和 `parse_status`。所有 ID 均为字符串，时间返回 UTC；缺少发件人资料或发送时间时为 `null`，空数组为 `[]`。`parse_status` 为 `parsed` 或 `partial`，未知消息类型不等于解析失败。

附件项含 `asset_key`、`kind`、`status`、`url`、`content_type`、`byte_size`。只有成功转存的附件提供 URL，`pending` 或 `failed` 附件仍返回且 URL 为 `null`。头像同样只提供转存 URL；不回退到在线原链接。响应不包含原始消息 JSON、Cookie 或内部错误文本。

## 定位消息与查看上下文

```http
GET /api/v1/weibo/groups/5152823595766382/messages/5357963156128139/context?before_limit=20&after_limit=20&highlight_q=%E6%95%B0%E6%8D%AE%E5%BA%93
```

返回目标消息及其最近的前后聊天，按数值消息 ID 升序排列。两侧各默认 20 条，允许 0～50 条；目标消息始终出现一次。同群目标不存在返回 `404`，ID、数量或高亮语法错误返回 `400`。

```json
{
  "message": "weibo message context",
  "data": {
    "anchor_message_id": "5357963156128139",
    "items": [],
    "has_more_before": true,
    "has_more_after": false
  }
}
```

`items` 使用相同消息结构和服务端 `text_matches`。`highlight_q` 只高亮，省略时没有高亮；上下文不筛选消息，不接受 `q`、发件人、时间、附件或列表分页参数，传入这些参数返回 `400`。

前后标志只描述本地已存档消息，即使某侧数量为 0 也会判断该侧是否有更多消息。继续滚动时调用 `/messages`：清空搜索和筛选，携带 `highlight_q`，用窗口最小或最大 ID 作为方向游标。两侧可持续加载到存档边界，不增加独立时间线接口。

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

同一群的所有调用由调用方串行执行，包括来自不同进程的调用。服务端现有 `MigrateDB` 会通过 AutoMigrate 建立存档及阅读状态表；独立调用者先执行 `db.AutoMigrate(&weibochat.Message{}, &weibochat.SyncState{}, &weibochat.Asset{}, &weibochat.User{})` 并检查错误。

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

## 收藏块

收藏属于当前登录用户。微博 API 同时使用管理员检查和现有 `InjectUser`，从 Authelia 的
`Remote-User` 取得所有者，不接受客户端传入所有者。所有 ID 均为字符串。

收藏块包含标题、标签及多条消息，可跨群追加消息。同一消息可以加入多个块，在同一块中只出现一次。
保存的是消息引用，不复制正文和附件。移除最后一条消息保留空块；删除块也不会删除存档。

```http
POST /api/v1/weibo/bookmark-blocks
Content-Type: application/json

{"title":"数据库讨论","tags":["技术"],"messages":[{"group_id":"5152823595766382","message_id":"5357963156128139"}]}
```

创建返回 `201`，`data` 含 `id`（UUID）、`title`、`tags`、`message_count`、`created_at` 和 `updated_at`。
标题默认空，去除首尾空白后最多 200 个 Unicode 码点；标签最多 20 个，每个去除首尾空白后为
1～50 个码点，按完整名称去重、区分大小写。创建和追加每次接受 1～100 条消息引用，去重后写入。
任何引用不存在则返回 `404`，本次操作不产生部分写入。

| 方法与路径（省略 `/api/v1/weibo`）                                 | 行为                                                |
| ------------------------------------------------------------------ | --------------------------------------------------- |
| `GET /bookmark-blocks`                                             | 当前用户收藏块，按更新时间倒序、UUID 倒序分页       |
| `GET /bookmark-blocks/:block_id`                                   | 收藏块元信息                                        |
| `PATCH /bookmark-blocks/:block_id`                                 | 修改 `title`、`tags`；省略表示不改，`tags: []` 清空 |
| `DELETE /bookmark-blocks/:block_id`                                | 删除块及其消息关系、标签                            |
| `GET /bookmark-blocks/:block_id/messages`                          | 块内消息，按发送时间升序分页                        |
| `POST /bookmark-blocks/:block_id/messages`                         | 追加消息，body 为 `{"messages":[...]}`              |
| `DELETE /bookmark-blocks/:block_id/messages/:group_id/:message_id` | 移除一条消息关系                                    |
| `GET /bookmark-tags`                                               | 当前用户的标签，`data.items` 为名称数组             |
| `GET /groups/:group_id/messages/:message_id/bookmark-blocks`       | 收录该消息的个人收藏块，使用相同块列表分页          |

块列表接受 `title` 字面子串（仅 ASCII 字母忽略大小写）、可重复的 `tag`（多个标签取 AND），
以及 `limit`、`cursor`。不搜索块内正文。列表 `data` 为 `{items: [...], next_cursor: "..."}`，
结束时 `next_cursor` 为 `null`。`limit` 默认 50，范围 1～100；后续请求保留筛选条件并传回游标。

块内消息也接受 `limit`、`cursor`，返回相同分页包装，`items` 使用 `MessageResponse`；可传
`highlight_q`，继续由服务端计算高亮，不影响成员。排序为发送时间升序，同时间按数值消息 ID、
数值群 ID 升序；缺失时间的消息在最后，仍以消息 ID 和群 ID 稳定排序。游标独立于群内聊天游标。
使用每条消息的群 ID 和消息 ID 调用 `/context`，即可跳回原群语境。

消息列表、上下文、收藏块消息均增加 `bookmark_count`，表示当前用户有多少个收藏块收录该消息。
收藏不是布尔开关，增加收藏应选择新建块或已有块。其他用户的块无法读取或修改，统一返回 `404`。
追加重复消息和移除不存在的关系是幂等操作，不改变块更新时间；有效增删消息或编辑元信息会更新时间。
追加、移除、删除成功返回 `204`，无响应正文。

AutoMigrate 新增 `weibo_chat_bookmark_block`、`weibo_chat_bookmark_message` 和
`weibo_chat_bookmark_tag`，不改变原始消息或 OSS 数据。

## 星标与拉黑

微博接口沿用管理员访问规则，并通过现有 `InjectUser` 取得站点登录用户名。星标和拉黑属于当前登录用户，按微博 UID 跨群生效，不修改共享用户资料或存档内容。

```http
PATCH /api/v1/weibo/users/123/preference
Content-Type: application/json

{"is_starred":true}
```

请求可包含 `is_starred`、`is_blocked`，至少提供一个布尔值；省略字段保留原值。两个状态独立保存：拉黑优先，但解除拉黑后原星标仍生效。响应为最新发件人资料及两个状态；没有用户资料时仍返回 UID。

```http
GET /api/v1/weibo/user-preferences?starred=true&limit=50
GET /api/v1/weibo/user-preferences?blocked=true&after_uid=123
```

偏好列表仅返回至少一个状态为 true 的用户。`starred`、`blocked` 可选，接受 `true` 或 `false`，同时提供时取 AND。按数值 UID 升序分页，`limit` 默认 50、范围 1～100。返回 `{ "items": [], "next_after_uid": null }`，有下一页时携带最后一个 UID 作为下次 `after_uid`。发件人字段含 `is_starred`、`is_blocked`。

消息列表增加 `starred_only=true`，与现有搜索、时间、发件人和附件条件取 AND；没有星标用户时返回空列表。通过 `before_message_id` / `after_message_id` 配合 `limit=1` 可定位上一条、下一条星标消息，再使用 `/context` 展示完整语境。同一星标用户连续发出的消息分别计为一个跳转位置。

每条消息新增 `is_sender_starred`、`is_sender_blocked`；即使 `sender` 为 null，也提供正确状态。普通消息列表和上下文保留拉黑用户的消息位置，由前端显示占位行并隐藏正文、链接与附件，不加载媒体。拉黑是阅读偏好，不是访问权限，也不会删除消息。正文搜索 `q` 非空或开启 `starred_only` 时排除拉黑用户；只有 `highlight_q` 不触发排除。上下文拒绝 `starred_only`，始终保留完整语境。

`weibo_chat_user_preference` 通过 AutoMigrate 创建，以 `(owner_username, uid)` 为主键。读取消息时批量取得当前用户偏好，不逐条查询。
