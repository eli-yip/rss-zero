package tkblog

import "regexp"

// reArchiveFanSite matches a tkblog fan-site permalink,
// tombkeeper.io/{xfocus|baidu}/{id}. The category path segment keeps it from
// colliding with the weibo matcher (tombkeeper.io/weibo/{digits}) — different
// second path segment, so the two never claim the same link.
var reArchiveFanSite = regexp.MustCompile(`tombkeeper\.io/(xfocus|baidu)/([0-9A-Za-z]+)`)

// BlogArchiveKey extracts (category, id) from a tkblog fan-site link, or ok=false
// when link is not one.
func BlogArchiveKey(link string) (category, id string, ok bool) {
	if m := reArchiveFanSite.FindStringSubmatch(link); m != nil {
		return m[1], m[2], true
	}
	return "", "", false
}

// IsBlogArchiveLink reports whether link is a tkblog fan-site archive link.
func IsBlogArchiveLink(link string) bool {
	_, _, ok := BlogArchiveKey(link)
	return ok
}
