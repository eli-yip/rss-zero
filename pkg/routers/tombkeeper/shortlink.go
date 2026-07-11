package tombkeeper

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var tcnRe = regexp.MustCompile(`https?://t\.cn/[A-Za-z0-9]+`)

// isViewPic reports whether a url_info entry is a "查看图片" reference — the reposter's
// own image attached on a 带图转发 repost, carried on a photo.weibo.com H5 page.
func isViewPic(e PostLink) bool {
	return e.URLType == 39 && strings.Contains(e.URLTitle, "查看图片")
}

// isWeiboTextLink reports whether a url_info entry is a link to tombkeeper's own
// weibo (used to decide archive-link + inline-quote rendering).
func isWeiboTextLink(e PostLink) bool {
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

func plainLink(e PostLink) string {
	title := e.URLTitle
	if title == "" {
		title = "网页链接"
	}
	return fmt.Sprintf("[%s](%s)", title, e.LongURL)
}
