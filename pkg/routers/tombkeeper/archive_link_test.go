package tombkeeper

import "testing"

func TestWeiboArchiveMid(t *testing.T) {
	cases := []struct {
		link    string
		wantMid string
		wantOK  bool
	}{
		{"https://weibo.com/detail/5312665532239202", "5312665532239202", true},
		{"https://tombkeeper.io/weibo/5310878589392289", "5310878589392289", true},
		{"weibo.com/detail/123", "123", true},
		{"https://weibo.com/1401527553/R5pVD1Ek5", "5312913130393333", true},      // tombkeeper uid/bid permalink
		{"https://weibo.com/1401527553/3992057061583495", "3992057061583495", true}, // tombkeeper uid/mid permalink (numeric form)
		{"https://weibo.com/2803301701/R5pVD1Ek5", "", false},                     // non-tombkeeper uid: not claimed
		{"https://example.com/x", "", false},                                      // not a weibo link
	}
	for _, c := range cases {
		got, ok := WeiboArchiveMid(c.link)
		if got != c.wantMid || ok != c.wantOK {
			t.Errorf("WeiboArchiveMid(%q) = %q, %v; want %q, %v", c.link, got, ok, c.wantMid, c.wantOK)
		}
	}
}
