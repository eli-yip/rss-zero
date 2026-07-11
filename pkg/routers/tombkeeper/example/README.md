# tombkeeper 结构化内容与渲染规则（SSOT）

本目录保存 tombkeeper 微博的真实 JSON 夹具；本文件规定字段语义、摄取边界与读时渲染规则。
目标是让抓取只产生可持久化的结构化内容，让 RSS 与归档从同一份内容快照纯函数生成 Markdown。

## 1. 页面提取与时间线成员

列表页包含两份互补信息：Next.js RSC flight 中有完整博文对象，SSR 的「详情」链接给出当前页
真正的时间线 id。`ExtractTimelinePage` 在 tombkeeper 包内汇合二者，返回：

- `Entries`：按「详情」链接顺序排列、且能在 flight 中找到的时间线博文；
- `EmbeddedPosts`：flight 中存在但不属于当前页时间线的博文，例如转发原文；
- `MissingEntries`：有「详情」链接但没有对应 flight 对象的 id，用于告警。

`Post.InTimeline` 表示该 id **曾经**出现在列表页「详情」条目中。写入只允许
`false → true`，详情页或内嵌对象的后续写入不能把它改回 false。RSS 只选择该字段为 true 的博文。

flight 提取先拼接全部 `self.__next_f.push` 分片，再解析 `$D` 日期与 `$ref`。`T<hexlen>,正文`
按声明的字节长度读取，避免正文里的换行被误判成 flight 行边界。时间无效或对象非法时不入库；若它
同时出现在「详情」列表中，则计入 `MissingEntries`。

## 2. 持久化的结构化内容

`tombkeeper_post` 保存：数字 id、bid、作者 uid/昵称、发布时间、正文纯文本、有序图片 id、
有序 `PostLink`、按 H5 长 URL 索引的图片 id、直接转发目标 id，以及 `in_timeline`。不保存
`title`、`text_markdown`、引用块、页脚、视频展示行或原始 JSON。

`PostLink` 保留短链、标题、长 URL、类型等渲染所需字段。视频、微博正文链接与「查看图片」都在
读时按这些字段分类，摄取时不生成 Markdown。

`tombkeeper_object` 是图片资产缓存，以裸 pic id 为键，保存对象存储键、实际来源 URL、provider
与成功/放弃状态；它不属于某一篇帖子。裸 pic id 按既有 sinaimg/CDN 候选下载，完整 URL 仍受 host
白名单约束。成功时渲染 OSS URL，失败时渲染失败提示或原始链接。

## 3. 摄取规则

`TimelineImporter.Import` 负责页面提取、直接依赖补全、图片求取和原子 upsert，不生成标题或
Markdown：

1. 时间线博文以 `InTimeline=true` 写入；同页内嵌博文以 false 写入。
2. 每个根帖只补一层直接依赖：缺失的 `RetweetPostID`，以及 tombkeeper 本人的「微博正文」链接。
3. 不递归补全依赖帖自己的链接/转发，避免请求树无界增长。
4. 普通 `pics` 和已解析的 H5 pic id 都写入共享图片资产表。

### H5「查看图片」缓存

`photo.weibo.com/h5/repost/reppic_id/...` 是页面 URL，不是图片 id。解析结果按完整 `LongURL`
存入 `H5ImageIDsByURL`，区分三种状态：

- key 不存在：尚未求取，或上次网络/解析失败；以后允许重试；
- key 存在且数组为空：请求成功但页面没有图片；以后不再重复请求；
- key 存在且数组非空：已经得到 pic id；以后直接复用。

并发 upsert 的合并顺序是 absent < empty < nonempty，因此旧请求不能覆盖已经成功取得的 id。

## 4. 内容装配与纯 Markdown 渲染

`ContentLoader.Load(roots)` 一次装配自包含的 `ContentSnapshot`：根帖、它们的一层直接转发/正文
链接目标，以及这些帖子需要的图片资产。数据库访问止于 loader。

`RenderMarkdown(postID, snapshot, serverBaseURL)` 只读取参数，不访问数据库、网络、全局配置或当前
时间，也不修改快照。RSS 与归档共用该函数；RSS 的 title 在读取时把正文空白折叠后取前 10 个 rune。

渲染约定：

- 正文按纯文本转义 Markdown 控制字符，`#话题#` 与 `@用户` 保持文字；
- 普通图片按 `Pics` 顺序追加 `![微博图片 N](...)`；
- 视频由链接标题或 `video.weibo.com` 长 URL 识别；正文已展开同一 URL 时不重复追加；
- tombkeeper 本人的「微博正文」短链在根帖中替换为归档链接，并在文末引用目标博文；只渲染一层；
- 显式转发由 `RetweetPostID` 引用目标博文，引用块末尾显示目标的北京时间；只渲染一层；
- H5「查看图片」在正文中替换为标号链接，并在转发引用块前展示；未取得图片时保留原 H5 链接；
- item link 指向 `https://weibo.com/{uid}/{bid}`；RSS/归档各自在纯正文之外添加所需页脚。

微博数字 mid 与 bid 的转换使用字母表
`0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ`：mid 从右按 7 位分组转 base62，
bid 从右按 4 字符分组逆转。链接目标统一先转成数字 id 再查询。

## 5. tombkeeper 账号

| uid          | 昵称       |
| ------------ | ---------- |
| `1401527553` | tombkeeper |
| `6827625527` | t0mbkeeper |

只有上述 uid 的「微博正文」链接会作为直接内容依赖补全；其他微博与外部链接按普通链接渲染。

## 6. 夹具覆盖

| 文件                           | 覆盖点                                                |
| ------------------------------ | ----------------------------------------------------- |
| `single_image.json`            | 单张裸 pic id                                         |
| `multi_image.json`             | 多图顺序                                              |
| `plain_text.json`              | 多个「微博正文」链接、按短链匹配与一层引用            |
| `retweet_with_original.json`   | 转发原文同页                                          |
| `retweet_original_absent.json` | 转发原文缺页，直接依赖回退抓取                        |
| `view_pic_retweet.json`        | H5 带图转发；转发者图片在引用块前、原文图片在引用块内 |
| `video.json`                   | 视频来自链接字段而非独立 `video_url`                  |

转发夹具的 `repost` / `original` 包装与 `_note` 只用于说明和测试，不是上游微博字段。
