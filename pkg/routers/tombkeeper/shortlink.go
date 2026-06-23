package tombkeeper

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/eli-yip/rss-zero/pkg/render"
)

var tcnRe = regexp.MustCompile(`https?://t\.cn/[A-Za-z0-9]+`)

// processShortLinks expands the t.cn short links in text using url_info. At
// depth 0, links to tombkeeper's own weibo ("微博正文") become an inline archive
// link plus a tail quote block inlining the linked post; every other link becomes
// a plain [title](long_url). At depth >= 1 (already inside an inlined post) all
// links are rendered plainly — inlining is limited to one layer. viewPicURLs holds
// the original's rehosted images (for a "查看图片" repost), so the in-text 查看图片
// link is replaced in place by a labeled link to the same image displayed nearby.
func (r *Renderer) processShortLinks(text string, urlInfo []URLInfoEntry, depth int, viewPicURLs []string) (string, []string) {
	if len(urlInfo) == 0 {
		return text, nil
	}
	byShort := make(map[string]URLInfoEntry, len(urlInfo))
	for _, e := range urlInfo {
		if e.ShortURL != "" {
			byShort[e.ShortURL] = e
		}
	}

	var tailQuotes []string
	n := 0
	viewPicN := 0
	newText := tcnRe.ReplaceAllStringFunc(text, func(tok string) string {
		e, ok := byShort[tok]
		if !ok {
			return tok
		}
		// "查看图片" carries the reposter's own 正文 image (also displayed before the
		// quote). Replace it in place with a labeled link to that same rehosted image so
		// the sentence stays coherent, sharing the image's number. If the image could
		// not be resolved, keep a clickable link to the original photo.weibo.com page
		// rather than dropping it.
		if isViewPic(e) {
			if viewPicN < len(viewPicURLs) {
				viewPicN++
				return fmt.Sprintf("[微博图片 %d](%s)", viewPicN, viewPicURLs[viewPicN-1])
			}
			return fmt.Sprintf("[查看图片|原始链接](%s)", e.LongURL)
		}
		if depth == 0 && isWeiboTextLink(e) {
			if _, bid := parseWeiboLong(e.LongURL); bid != "" {
				if targetMid, err := BidToMid(bid); err == nil {
					if body, sn, ok := r.materializePost(targetMid); ok {
						n++
						tailQuotes = append(tailQuotes,
							quoteBlock(fmt.Sprintf("微博正文%d @%s", n, sn), body))
						rssURL := render.BuildArchiveLink(r.serverURL, e.LongURL)
						return fmt.Sprintf("[微博正文%d](%s)", n, rssURL)
					}
				}
			}
		}
		return plainLink(e)
	})
	return newText, tailQuotes
}

// isViewPic reports whether a url_info entry is a "查看图片" reference — the reposter's
// own image attached on a 带图转发 repost, carried on a photo.weibo.com H5 page.
func isViewPic(e URLInfoEntry) bool {
	return e.URLType == 39 && strings.Contains(e.URLTitle, "查看图片")
}

// isWeiboTextLink reports whether a url_info entry is a link to tombkeeper's own
// weibo (used to decide archive-link + inline-quote rendering).
func isWeiboTextLink(e URLInfoEntry) bool {
	if e.URLType != 0 || e.URLTitle != "微博正文" {
		return false
	}
	uid, _ := parseWeiboLong(e.LongURL)
	return IsTombkeeperUID(uid)
}

// parseWeiboLong splits a https://weibo.com/{uid}/{bid} long_url.
func parseWeiboLong(longURL string) (uid, bid string) {
	u, err := url.Parse(longURL)
	if err != nil {
		return "", ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

// Archive-link matchers, shared by the archive controller's dispatcher and its
// handler so the two cannot drift. Compiled once.
var (
	reArchiveDetail  = regexp.MustCompile(`weibo\.com/detail/(\d+)`)
	reArchiveFanSite = regexp.MustCompile(`tombkeeper\.io/weibo/(\d+)`)
	reArchiveUIDBid  = regexp.MustCompile(`weibo\.com/(\d+)/([A-Za-z0-9]+)`)
)

// WeiboArchiveMid extracts the tombkeeper weibo mid from an archive link, or
// returns ok=false when the link is not a tombkeeper-archivable weibo. The
// uid/bid permalink form is accepted only when uid is a tombkeeper account and
// its bid converts to a mid, so arbitrary weibo.com/{uid}/{bid} links (other
// users, profile sub-paths) are not claimed by the tombkeeper handler.
func WeiboArchiveMid(link string) (mid string, ok bool) {
	if m := reArchiveDetail.FindStringSubmatch(link); m != nil {
		return m[1], true
	}
	if m := reArchiveFanSite.FindStringSubmatch(link); m != nil {
		return m[1], true
	}
	if m := reArchiveUIDBid.FindStringSubmatch(link); m != nil {
		if !IsTombkeeperUID(m[1]) {
			return "", false
		}
		if mid, err := BidToMid(m[2]); err == nil {
			return mid, true
		}
	}
	return "", false
}

// IsWeiboArchiveLink reports whether link is a tombkeeper-archivable weibo link.
func IsWeiboArchiveLink(link string) bool {
	_, ok := WeiboArchiveMid(link)
	return ok
}

func weiboDetailURL(mid string) string { return "https://weibo.com/detail/" + mid }

// WeiboPostURL builds the canonical weibo permalink in uid/bid form, e.g.
// https://weibo.com/1401527553/R5pVD1Ek5. Falls back to the detail/mid form when
// the bid is unknown.
func WeiboPostURL(uid, bid, mid string) string {
	if uid != "" && bid != "" {
		return "https://weibo.com/" + uid + "/" + bid
	}
	return weiboDetailURL(mid)
}

// FanSiteURL builds the tombkeeper.io mirror ("粉丝站") link for a weibo id.
func FanSiteURL(mid string) string { return siteBaseURL + "/weibo/" + mid }

func plainLink(e URLInfoEntry) string {
	title := e.URLTitle
	if title == "" {
		title = "网页链接"
	}
	return fmt.Sprintf("[%s](%s)", title, e.LongURL)
}
