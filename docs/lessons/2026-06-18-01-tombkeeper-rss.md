# LESSON: tombkeeper 微博 RSS 订阅源

- 日期：2026-06-18
- SPEC：[`../specs/2026-06-18-01-tombkeeper-rss.md`](../specs/2026-06-18-01-tombkeeper-rss.md)
- PLAN：[`../plans/2026-06-18-01-tombkeeper-rss.md`](../plans/2026-06-18-01-tombkeeper-rss.md)
- 解析 SSOT：[`../../pkg/routers/tombkeeper/example/README.md`](../../pkg/routers/tombkeeper/example/README.md)

实现一个消费 `tombkeeper.io` 镜像站的微博订阅源。记录调研与实现中**非显而易见**的点。

## 调研：数据其实藏在哪里

- `tombkeeper.io` 是 Next.js SSR 站点。`<article>` 里渲染好的正文 HTML **不含图片**；真正
  的结构化数据嵌在页面的 **RSC flight 负载**（`self.__next_f.push([1,"…"])` 多分片）里。
- `text` 字段是**纯文本**（含 `\n`、内联 `#话题#`/`@用户`、全角空格），不是 HTML。所以管线
  是「纯文本 + 结构化字段 → Markdown」，不是 HTML→Markdown。
- 图片只在 `pics`，**多为裸 pic id（无扩展名/无 host）**，少数为完整 sinaimg URL。
- **视频与 `t.cn` 短链在 `url_info`（一个 `$ref`），不在 `video_url`（实测恒空）**。`url_type`
  不可靠（39 既是"微博视频"也是"查看图片"），靠 `url_title`/`long_url` 判定。
- **转发原文与转发帖同页嵌入**（实测原文 9/9 同页），所以内联转发常态零额外请求。

## 关键决策与其依据

- **flight 提取**：必须先把所有 `__next_f.push` 分片**按序拼接**再做花括号配平，单分片内
  brace-match 会因对象跨分片而断裂。`created_at` 带 `$D` 前缀需剥离；`url_info` 的 `$ref`
  需按 flight 行解引用。
- **mid ↔ bid base62 双向**：可离线、确定、无损转换。**字母表小写在前** `0-9a-zA-Z`（大写在
  前会全错）；用 7 对真实样本验证。决策以**数字 mid 为全局主键**，链接里的 bid 一律先转 mid。
- **哪些是 feed 条目**：列表页的 5 条「时间线帖」= 页面里 5 个 `/weibo/{id}` **详情链接**；
  嵌入的转发原文不带详情链接。用这个信号区分，避免把他人/旧的嵌入原文当成 feed 条目。这是
  从 flight 对象本身无法区分的，必须借 SSR HTML 的详情链接。
- **图片下载**：裸 id 参考 image-seeker 展开 sinaimg 主机变体 + 第三方代理候选，**并发探测**
  取首个 200（带 `Referer: https://weibo.com`）。全失败则标记 `object.status=1` 并在正文
  **保留原始链接**（不丢图）。
- **隐式引用（链接型）vs 转发（retweet）**：两者都内联，但触发源不同。链接型只对 tombkeeper
  自己的「微博正文」链接展开（uid 判定），且把目标帖**抓取入库**以备归档页渲染；两类内联都
  **只 1 层**，避免递归与请求放大。
- **不设检测表**：是否 tombkeeper 由 uid 直接判定；避免重复抓取由「抓到即入库」保证——每条
  链接只在宿主帖首次入库时处理一次。

## 实现踩点

- **GORM 自动时间戳的陷阱**：字段名 `CreatedAt`/`UpdatedAt` 会被 GORM 当作自动维护的
  插入/更新时间。微博**发布时间**要存到 `created_at` 列又不想被自动覆盖，就把 Go 字段命名为
  `PostTime`（映射 `column:created_at`），另用 `CreatedDBAt`/`UpdatedDBAt`（带 `autoCreateTime`/
  `autoUpdateTime` tag）存库内时间。
- **`SaveStream` 拥有 body**：minio 实现里 `defer stream.Close()`，所以把 `resp.Body` 交给
  `SaveStream` 后**不要再 defer 关闭**（否则双关）；并发探测里**落败的成功响应**要显式关 body。
- **归档页是自然派生**：单条微博归档页直接返回已存的 `text_markdown`（已转 OSS 图、已展开
  链接），无需重渲染；接入现有 `/api/v1/archive/:url` 的 URL 正则分派即可。HTML 不显示标题
  （`title` 仅用于 RSS item 与 `<title>`）。
- **接线**：`fileService` 原先在 `setupCronCrawlJob` 之后才创建，需上移并作为入参传入，cron
  job 才能拿到 OSS。
- **lint**：`autocorrect` 会改注释/文档里的 CJK 间距，改完 md 还要 `dprint fmt` 重排表格；
  `golangci-lint` 的 modern-go 会要求 `range len(n)`/`max(...)`/`strings.SplitSeq`。

## 验证

- 单测覆盖：mid/bid 双向（真实 7 对）、flight 提取（跨分片 + `$ref` + 去重）、图片候选生成、
  渲染（单图/多图/全失败保留原链/视频/转发内联）、短链（微博正文识别 + 编号 + 文末引用 + 普通
  链接 + depth-1 降级）、归档 URL 解析。网络与 OSS 全用注入 stub。
- 端到端在真实页面验证：5 条时间线帖正确抽取与渲染，转发以引用块内联，产出合法 Atom。
- 预存失败 `TestExtractAnswerID`（zhihu，master 即失败）与本特性无关，见 `docs/TODO.md`。

## 对抗式评审结果

5 维并行评审（正确性/并发/SSOT 一致性/集成/安全）共 29 条初 findings，逐条对抗式验证后**仅 2 条
确认为真**，均已修复：

- **限速 goroutine 泄漏（medium）**：`NewRequestService` 在每次 `Crawl`（每小时）里新建，其无缓冲
  限速 goroutine 无停止信号，Crawl 返回后永久阻塞并 pin 住整个 service——每小时漏 1 个。修复：
  `Requester.Close()`（`sync.Once`+`done` 通道 select）+ `Crawl` 里 `defer req.Close()`。
- **盲打 SSRF（low）**：完整 URL 形态的 `pics` 来自镜像站、原样服务端抓取且无 host 校验。修复：
  仅抓 host 在允许名单内的完整 URL，并把图片下载的重定向 host 也约束在名单内。纵深加固（拨号层
  阻断私有网段）按低优先记入 TODO。
- 其余 27 条经验证为误报/低置信驳回——实现整体稳健。
