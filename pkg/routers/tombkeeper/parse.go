package tombkeeper

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/md"
)

// Renderer turns extracted RawPosts into stored Posts: it converts the plain text
// to Markdown, rehosts images to OSS, expands url_info short links, inlines
// retweet originals and linked weibo-text posts. Dependencies are injected so the
// rendering can be unit-tested without network or OSS.
type Renderer struct {
	req       Requester
	file      file.File
	db        DB
	serverURL string
	logger    *zap.Logger
}

func NewRenderer(req Requester, f file.File, db DB, serverURL string, logger *zap.Logger) *Renderer {
	return &Renderer{req: req, file: f, db: db, serverURL: serverURL, logger: logger}
}

// Render builds the stored Post for one timeline weibo. pageMap holds all posts
// extracted from the same page (so a retweet original can be inlined without an
// extra request). Images and any newly-fetched linked posts are persisted as a
// side effect.
func (r *Renderer) Render(raw RawPost, pageMap map[string]RawPost) (*Post, error) {
	if _, err := strconv.ParseInt(raw.ID, 10, 64); err != nil {
		return nil, fmt.Errorf("bad post id %q: %w", raw.ID, err)
	}
	body := r.renderContent(raw, pageMap, 0)
	return r.buildPost(raw, body), nil
}

// renderContent assembles the markdown body of one post: text, images, video,
// retweet quote, and linked-post quotes. depth limits inlining to one layer.
func (r *Renderer) renderContent(raw RawPost, pageMap map[string]RawPost, depth int) string {
	mid, _ := strconv.ParseInt(raw.ID, 10, 64)

	// Resolve the retweet original up front (depth 0 only). Its own images stay inside
	// the quote (rendered by the recursive call below).
	var orig *RawPost
	if depth == 0 && raw.RetweetID != "" {
		orig = r.resolveOriginal(raw.RetweetID, pageMap)
	}

	// A 带图转发 (repost-with-image) carries the reposter's OWN image via a "查看图片"
	// url_info whose long_url is a photo.weibo.com H5 page. Resolve it to that image
	// (the post's 正文 image), so the in-text 查看图片 short link becomes a labeled link
	// and the same image is displayed before the quote — distinct from the original's
	// own image, which is rendered inside the quote.
	var viewPicURLs []string
	if depth == 0 {
		viewPicURLs = r.resolveViewPics(raw.URLInfo, mid)
	}

	body, tailQuotes := r.processShortLinks(escapeMarkdown(raw.Text), raw.URLInfo, depth, viewPicURLs)

	sections := []string{body}
	sections = append(sections, imageEmbeds(r.rehostImages(raw.Pics, mid))...)
	// Append the video link only if it was not already expanded inline by a t.cn
	// short link (otherwise the same video shows twice — the stray extra line).
	if v, vurl := videoLink(raw.URLInfo); v != "" && !strings.Contains(body, vurl) {
		sections = append(sections, v)
	}

	// The reposter's own 正文 image (from 查看图片) is displayed right before the quote.
	sections = append(sections, imageEmbeds(viewPicURLs)...)
	if orig != nil {
		quote := r.renderContent(*orig, nil, 1)
		sections = append(sections, quoteBlock("转发 @"+orig.ScreenName, quote))
	}
	sections = append(sections, tailQuotes...)

	return strings.TrimRight(md.Join(sections...), "\n")
}

// resolveViewPics resolves the reposter's own attached image(s) from any "查看图片"
// url_info entries (带图转发): it fetches each photo.weibo.com H5 page and rehosts the
// sinaimg image(s) to OSS, returning their markdown URLs in order. Empty when there is
// no 查看图片 entry or it cannot be resolved — the in-text link then falls back to the
// original H5 link (see processShortLinks). Images are attributed to postID (the
// reposter's post), since they are the reposter's own.
func (r *Renderer) resolveViewPics(urlInfo []URLInfoEntry, postID int64) []string {
	var urls []string
	for _, e := range urlInfo {
		if !isViewPic(e) {
			continue
		}
		picIDs, err := r.req.GetReppic(e.LongURL)
		if err != nil {
			r.logger.Warn("failed to resolve 查看图片 reppic page", zap.String("long_url", e.LongURL), zap.Error(err))
			continue
		}
		for _, id := range picIDs {
			u, err := saveImage(r.req, r.file, r.db, postID, id, r.logger)
			if err != nil {
				r.logger.Warn("failed to rehost reppic image", zap.String("pic", id), zap.Error(err))
				continue
			}
			urls = append(urls, u)
		}
	}
	return urls
}

func (r *Renderer) buildPost(raw RawPost, body string) *Post {
	mid, _ := strconv.ParseInt(raw.ID, 10, 64)
	_, videoURL := videoLink(raw.URLInfo)
	// A zero CreatedAt means created_at failed to parse (parseFlightTime fell back
	// to the zero time). Use now instead, so the feed gets a valid timestamp rather
	// than a year-0001 <updated> that readers sort to the bottom or reject.
	postTime := raw.CreatedAt
	if postTime.IsZero() {
		r.logger.Warn("post created_at missing or unparseable, using current time", zap.String("id", raw.ID))
		postTime = time.Now()
	}
	return &Post{
		ID:           mid,
		Bid:          raw.Bid,
		AuthorID:     raw.UserID,
		ScreenName:   raw.ScreenName,
		PostTime:     postTime,
		Title:        makeTitle(raw.Text),
		TextMarkdown: body,
		VideoURL:     videoURL,
		RetweetID:    raw.RetweetID,
		Raw:          raw.Raw,
	}
}

// rehostImages rehosts every pic in a pics field and returns the ordered list of
// resolved markdown URLs (the OSS copy, or the original sinaimg link for an
// abandoned image). Rehosting is idempotent: an already-stored object is reused.
func (r *Renderer) rehostImages(pics string, mid int64) []string {
	var urls []string
	for _, p := range splitPics(pics) {
		u, err := saveImage(r.req, r.file, r.db, mid, p, r.logger)
		if err != nil {
			r.logger.Warn("failed to rehost image", zap.String("pic", p), zap.Error(err))
			continue
		}
		urls = append(urls, u)
	}
	return urls
}

// imageEmbeds renders resolved image URLs as numbered inline images
// "![微博图片 N](url)" (one per line). An inline embed (not a plain link) makes the
// image display directly in the reader while keeping the "微博图片 N" label as alt text.
func imageEmbeds(urls []string) []string {
	out := make([]string, len(urls))
	for i, u := range urls {
		out[i] = md.Image(fmt.Sprintf("微博图片 %d", i+1), u)
	}
	return out
}

// resolveOriginal returns the retweet original, preferring the same-page object
// and falling back to a single detail fetch.
func (r *Renderer) resolveOriginal(retweetID string, pageMap map[string]RawPost) *RawPost {
	if orig, ok := pageMap[retweetID]; ok {
		o := orig
		return &o
	}
	html, err := r.req.GetDetail(retweetID)
	if err != nil {
		r.logger.Warn("failed to fetch retweet original", zap.String("id", retweetID), zap.Error(err))
		return nil
	}
	posts, _ := ExtractPosts(html)
	for i := range posts {
		if posts[i].ID == retweetID {
			return &posts[i]
		}
	}
	return nil
}

// materializePost returns a tombkeeper post's rendered body for inline quoting,
// preferring the database and otherwise fetching it once (and persisting it, so
// the next crawl skips re-fetching). Only used for tombkeeper-own weibo-text
// links, so persisting it never introduces a non-tombkeeper feed item.
func (r *Renderer) materializePost(mid string) (body, screenName string, ok bool) {
	midInt, err := strconv.ParseInt(mid, 10, 64)
	if err != nil {
		return "", "", false
	}
	if exists, _ := r.db.PostExists(midInt); exists {
		if p, e := r.db.GetPost(midInt); e == nil {
			return p.TextMarkdown, p.ScreenName, true
		}
	}
	html, err := r.req.GetDetail(mid)
	if err != nil {
		return "", "", false
	}
	posts, _ := ExtractPosts(html)
	for _, p := range posts {
		if p.ID == mid {
			b := r.renderContent(p, nil, 1)
			_ = r.db.SavePost(r.buildPost(p, b))
			return b, p.ScreenName, true
		}
	}
	return "", "", false
}

func quoteBlock(header, body string) string {
	content := header
	if body != "" {
		content += "\n\n" + body
	}
	return md.Quote(content)
}

// splitPics returns the ordered, de-duplicated pic entries of a pics field.
func splitPics(pics string) []string {
	var out []string
	seen := make(map[string]struct{})
	for p := range strings.SplitSeq(pics, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		id := picIDOf(p)
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, p)
	}
	return out
}

// videoLink returns a markdown link to the weibo video carried in url_info (and
// its long_url), if any.
func videoLink(urlInfo []URLInfoEntry) (markdown, longURL string) {
	for _, e := range urlInfo {
		if strings.Contains(e.URLTitle, "微博视频") || strings.Contains(e.LongURL, "video.weibo.com") {
			return fmt.Sprintf("[微博视频](%s)", e.LongURL), e.LongURL
		}
	}
	return "", ""
}
