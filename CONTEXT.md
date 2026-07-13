# RSS-ZERO 内容归档

RSS-ZERO 把不友好来源的内容保存为可重放的结构化记录，并按需呈现为订阅源和归档页面。

## tombkeeper 微博

**博文（Post）**：
一条微博的结构化内容，包括作者、发布时间、正文、链接、图片和直接转发关系。
_Avoid_: Fact、Record、Item

**时间线条目（Timeline Entry）**：
在 tombkeeper.io 列表页中有「详情」入口的博文；只有时间线条目有资格进入 RSS。
_Avoid_: Timeline Post、Feed Post

**内嵌博文（Embedded Post）**：
随时间线页面载荷出现、用于解释另一条博文的完整微博对象；出现本身不证明它位于时间线。
_Avoid_: Referenced Fact、Extra Post

**图片资产（Image Asset）**：
博文引用的一张源图片及其转存成功或放弃的结果。
_Avoid_: Object、Media Object

**H5 图片索引（H5 Image Index）**：
从「查看图片」H5 URL 到有序图片 id 的映射；URL 已存在表示解析成功，即使结果为空。
_Avoid_: View Pics、H5 Result

**内容快照（Content Snapshot）**：
呈现一组博文所需的博文与图片资产的自包含只读集合。
_Avoid_: Render Dataset、Render Context

## 共享（读取期渲染）

zhihu 与 zsxq 的事实化重写沿用 tombkeeper 的「只存事实、读取期纯渲染」骨架，形成跨源共同词汇。

**内容快照（ContentSnapshot）**：
呈现一批内容所需的根行与其引用事实（object / question / article / author）的自包含只读集合；
不持久化，读取期由 loader 装配后喂给纯 renderer。
_Avoid_: RenderDataset、RenderContext

**内容装配器（ContentLoader）**：
各源一个，两阶段批量把一组根行与其引用装配成内容快照，避免 N+1；可查库，产物只读。
_Avoid_: 跨源 Library、FactReader

**待提交解析结果（Parse Result）**：
一条内容解析出的、待在同一事务内落库的全部事实（`TopicParseResult`、`PinParseResult` 等，
各源按需定义，不抽通用 interface）。
_Avoid_: 通用 Fact / Record interface

**transient 派生渲染（Transient Derived Render）**：
抓取期在内存里跑一次读取期同一个纯 renderer，把临时 Markdown 喂给 word_count、内容检测、
embedding、AI 标题等派生事实，产物**不落库**；用于保派生值不漂移，而非持久化正文。
_Avoid_: 抓取期保存 Markdown
