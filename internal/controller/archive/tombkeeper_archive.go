package archive

import (
	"errors"
	"fmt"
	"strconv"

	"gorm.io/gorm"

	tk "github.com/eli-yip/rss-zero/pkg/routers/tombkeeper"
)

// HandleTombkeeperWeibo renders a single tombkeeper weibo's archive page. The
// post markdown is already fully rendered (OSS images, expanded links), so it is
// returned verbatim. The HTML page shows no title (empty <title>), only the body.
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
	// Like the zhihu archive, append source links — here the original weibo and
	// the tombkeeper.io mirror ("粉丝站").
	footer := fmt.Sprintf("[微博](%s) · [粉丝站](%s)",
		tk.WeiboPostURL(post.AuthorID, post.Bid, mid), tk.FanSiteURL(mid))
	return &archiveResult{title: "", markdown: post.TextMarkdown + "\n\n" + footer}, nil
}
