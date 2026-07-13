package render

import (
	"errors"
	"fmt"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/render"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
)

// FullTextRenderer 把一条 topic 根行渲染成归档/导出用的完整文本（标题壳 + 正文 + 时间 + 原文链接）。
type FullTextRenderer interface {
	FullText(topic zsxqDB.Topic) (text string, err error)
	// LoadSnapshot 为一页 topic 批量装配一次快照，供 FullTextFromSnapshot 逐条渲染，
	// 避免 export 循环里每条 topic 各查一次侧表（feed 路径 zsxqRows 已是这个形状）。
	LoadSnapshot(topics []zsxqDB.Topic) (ContentSnapshot, error)
	// FullTextFromSnapshot 用调用方已装配好的快照渲染单条 topic 的完整文本。
	FullTextFromSnapshot(t zsxqDB.Topic, snapshot ContentSnapshot) (text string, err error)
}

// FullTextRenderService 读取期从 raw 渲染正文：装配快照走的是与 feed 同一条纯渲染路径，
// 不再吃冻结的 topic.Text。
type FullTextRenderService struct {
	loader ContentLoader
	mdFmt  *md.MarkdownFormatter
}

// NewFullTextRenderService 注入批量只读 reader（读取期传 db 服务），供 loader 装配快照。
func NewFullTextRenderService(reader ContentReader) FullTextRenderer {
	return &FullTextRenderService{loader: NewContentLoader(reader), mdFmt: md.NewMarkdownFormatter()}
}

func (m *FullTextRenderService) LoadSnapshot(topics []zsxqDB.Topic) (ContentSnapshot, error) {
	return m.loader.Load(topics)
}

func (m *FullTextRenderService) FullText(t zsxqDB.Topic) (text string, err error) {
	snapshot, err := m.loader.Load([]zsxqDB.Topic{t})
	if err != nil {
		return "", err
	}
	return m.FullTextFromSnapshot(t, snapshot)
}

// FullTextFromSnapshot 渲染单条 topic 的信封（标题 + 正文 + 时间 + 链接）。ParseTopic 会持久化
// 非 talk/q&a 的未知类型 topic（只存元数据+raw，不存侧表事实），RenderMarkdown 对这类
// topic 返回 ErrUnknownType；旧实现（topic.Text 列时代）对这类 topic 落库的是空字符串
// 正文，导出/web 仍照常输出信封（feed 路径另有 render.Support 前置过滤，跳过整条，
// 这里不是 feed 路径，不动）。故此处把 ErrUnknownType 降级为空正文，不当错误处理，
// 保持导出/web 旧行为字节级一致。
func (m *FullTextRenderService) FullTextFromSnapshot(t zsxqDB.Topic, snapshot ContentSnapshot) (text string, err error) {
	body, err := RenderMarkdown(t.ID, snapshot)
	if err != nil {
		if !errors.Is(err, ErrUnknownType) {
			return "", err
		}
		body = ""
	}

	title := render.TrimRightSpace(md.H1(BuildTitle(t)))
	timePart := zsxqTime.FmtForRead(t.Time)
	link := BuildLink(t.GroupID, t.ID)
	linkText := render.TrimRightSpace(fmt.Sprintf("[%s](%s)", link, link))
	text = md.Join(title, body, timePart, linkText)
	return m.mdFmt.FormatStr(text)
}
