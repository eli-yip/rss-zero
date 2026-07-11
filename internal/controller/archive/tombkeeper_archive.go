package archive

import (
	"errors"
	"fmt"
	"strconv"

	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/config"
	tk "github.com/eli-yip/rss-zero/pkg/routers/tombkeeper"
)

// HandleTombkeeperWeibo 从已存结构化内容渲染一篇归档微博。
func (h *Controller) HandleTombkeeperWeibo(link string) (*archiveResult, error) {
	mid, ok := tk.WeiboArchiveMid(link)
	if !ok {
		return nil, fmt.Errorf("no tombkeeper weibo id in link: %s", link)
	}
	midInt, err := strconv.ParseInt(mid, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("bad weibo mid %q: %w", mid, err)
	}
	post, err := h.tombkeeperDBService.GetPost(midInt)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("tombkeeper weibo %d not archived: %w", midInt, ErrArchiveNotFound)
		}
		return nil, fmt.Errorf("get tombkeeper post %d: %w", midInt, err)
	}
	content, err := tk.NewContentLoader(h.tombkeeperDBService).Load([]tk.Post{*post})
	if err != nil {
		return nil, fmt.Errorf("load tombkeeper content %d: %w", midInt, err)
	}
	markdown, err := tk.RenderMarkdown(post.ID, content, config.C.Settings.ServerURL)
	if err != nil {
		return nil, fmt.Errorf("render tombkeeper post %d: %w", midInt, err)
	}
	footer := fmt.Sprintf("[微博](%s) · [粉丝站](%s)",
		tk.WeiboPostURL(post.AuthorID, post.Bid, mid), tk.FanSiteURL(mid))
	return &archiveResult{title: "", markdown: markdown + "\n\n" + footer}, nil
}
