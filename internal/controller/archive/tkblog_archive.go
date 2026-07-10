package archive

import (
	"errors"
	"fmt"

	"gorm.io/gorm"

	tkblog "github.com/eli-yip/rss-zero/pkg/routers/tkblog"
)

// HandleTkblog renders a single tkblog post's archive page. Unlike weibo, blog
// posts carry a real title, so it is passed through as the page <title>. The
// stored markdown is plain escaped text; ArchiveFooter appends the Wayback
// original (when present) and the fan-site permalink.
func (h *Controller) HandleTkblog(link string) (*archiveResult, error) {
	category, id, ok := tkblog.BlogArchiveKey(link)
	if !ok {
		return nil, fmt.Errorf("no tkblog id in link: %s", link)
	}
	post, err := h.tkblogDBService.GetPost(category, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("tkblog post %s/%s not archived: %w", category, id, ErrArchiveNotFound)
		}
		return nil, fmt.Errorf("get tkblog post %s/%s: %w", category, id, err)
	}
	return &archiveResult{title: post.Title, markdown: post.TextMarkdown + "\n\n" + tkblog.ArchiveFooter(post)}, nil
}
