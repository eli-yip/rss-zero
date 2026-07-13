package tombkeeper

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
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

// parseWeiboLong splits a https://weibo.com/{uid}/{bid|mid} long_url. The second
// segment is usually a base62 bid but can also be a numeric mid; callers resolve
// it via weiboSegmentToMid.
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

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// weiboMblogidMaxLen 是任意 int64 mid 经 MidToBid 得到的 mblogid 最大长度：mid 按 7
// 位分组，每组 base62 至多 4 字符、最高组至多 3 字符 → 3+4+4 = 11。
const weiboMblogidMaxLen = 11

// weiboSegmentToMid 把 weibo 永久链接第二段解析成数字 mid。该段通常是 base62 bid，
// 但 weibo（及 tombkeeper.io 的 url_info）也会直接用数字 mid 形式。长度 > 11 的纯数字
// 段不可能是 bid（任何 int64 mid 的 bid ≤ 11 字符），故只能是数字 mid，直接返回，避免
// 被 BidToMid 误当 base62 解出乱码；较短的纯数字段与真实 bid 有歧义，仍走 BidToMid。
func weiboSegmentToMid(seg string) (string, error) {
	if len(seg) > weiboMblogidMaxLen && isAllDigits(seg) {
		return seg, nil
	}
	return BidToMid(seg)
}

// weiboLinkPostID 从 uid/{bid|mid} 长链解析出微博的数字 id。
func weiboLinkPostID(longURL string) (int64, bool) {
	_, seg := parseWeiboLong(longURL)
	mid, err := weiboSegmentToMid(seg)
	if err != nil {
		return 0, false
	}
	id, err := strconv.ParseInt(mid, 10, 64)
	if err != nil {
		return 0, false
	}
	return id, true
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
		if mid, err := weiboSegmentToMid(m[2]); err == nil {
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
