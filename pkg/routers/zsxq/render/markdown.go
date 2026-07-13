package render

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/render"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
)

// MarkdownRenderer 只保留抓取期外部文章 HTML→Markdown 归一化这一豁免（见 plan 决策 5）；
// topic 正文渲染已迁到读取期纯函数 RenderMarkdown。
type MarkdownRenderer interface {
	// Article converts a article html to markdown
	Article([]byte) (string, error)
}

type MarkdownRenderService struct {
	htmlToMarkdown render.HTMLToMarkdown
	mdFmt          *md.MarkdownFormatter
}

func NewMarkdownRenderService() MarkdownRenderer {
	s := &MarkdownRenderService{
		htmlToMarkdown: render.NewHTMLToMarkdownService(getArticleRules()...),
		mdFmt:          md.NewMarkdownFormatter(),
	}

	return s
}

// BuildLink builds official link for a zsxq topic.
func BuildLink(groupID, topicID int) string {
	return fmt.Sprintf("https://wx.zsxq.com/group/%d/topic/%d", groupID, topicID)
}

// BuildGroupLink builds the official link for a zsxq group.
func BuildGroupLink(groupID int) string {
	return fmt.Sprintf("https://wx.zsxq.com/group/%d", groupID)
}

// BuildTitle 从 topic 根行推导归档/导出标题：无标题回退到 topic id，精华加前缀。
func BuildTitle(t zsxqDB.Topic) string {
	titlePart := func() string {
		if t.Title == nil {
			return strconv.Itoa(t.ID)
		} else {
			return *t.Title
		}
	}()

	if t.Digested {
		titlePart = fmt.Sprintf("[%s]%s", "精华", titlePart)
	}

	return titlePart
}

var ErrUnknownType = errors.New("unknown type")

func (m *MarkdownRenderService) Article(article []byte) (text string, err error) {
	bytes, err := m.htmlToMarkdown.ConvertWithTimeout(article, render.DefaultTimeout)
	if err != nil {
		return "", err
	}

	if text, err = m.mdFmt.FormatStr(string(bytes)); err != nil {
		return "", err
	}

	return text, nil
}

var ErrNoText = errors.New("no text in topic")
