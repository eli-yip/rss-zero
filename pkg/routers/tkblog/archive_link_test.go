package tkblog

import "testing"

func TestBlogArchiveKey(t *testing.T) {
	cases := []struct {
		link         string
		wantCategory string
		wantID       string
		wantOK       bool
	}{
		{"https://tombkeeper.io/baidu/26ho1i8FcjS", "baidu", "26ho1i8FcjS", true},
		{"https://tombkeeper.io/xfocus/9kZ2xFocus", "xfocus", "9kZ2xFocus", true},
		{"tombkeeper.io/baidu/abc123", "baidu", "abc123", true},
		{"https://tombkeeper.io/weibo/5310878589392289", "", "", false}, // weibo link, not a blog article
		{"https://tombkeeper.io/baidu", "", "", false},                  // list page, no id
		{"https://example.com/baidu/x", "", "", false},                  // wrong host
	}
	for _, c := range cases {
		cat, id, ok := BlogArchiveKey(c.link)
		if cat != c.wantCategory || id != c.wantID || ok != c.wantOK {
			t.Errorf("BlogArchiveKey(%q) = %q,%q,%v; want %q,%q,%v",
				c.link, cat, id, ok, c.wantCategory, c.wantID, c.wantOK)
		}
		if IsBlogArchiveLink(c.link) != c.wantOK {
			t.Errorf("IsBlogArchiveLink(%q) = %v, want %v", c.link, !c.wantOK, c.wantOK)
		}
	}
}
