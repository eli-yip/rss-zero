package middleware

import (
	"path"
	"regexp"
	"testing"
)

func TestExtract(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"/rss/zhihu/canglimo", "canglimo"},
		{"/rss/zhihu/canglimo.atom", "canglimo"},
		{"/rss/zhihu/canglimo.atom/rss", "canglimo"},
		{"/rss/zhihu/canglimo.atom/feed", "canglimo"},
		{"/rss/zhihu/canglimo.com", "canglimo"},
		{"/rss/zhihu/canglimo/rss.com", "canglimo"},
		{"/rss/zhihu/canglimo/feed.com", "canglimo"},
	}
	re := regexp.MustCompile(`(/rss|/feed)?(\.com)?(\.atom)?(/rss|/feed)?$`)
	for _, c := range cases {
		got := re.ReplaceAllString(c.in, "")
		got = path.Base(got)
		if got != c.want {
			t.Errorf("Extract(%q) == %q, want %q", c.in, got, c.want)
		}
	}
}
