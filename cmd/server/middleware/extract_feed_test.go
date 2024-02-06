package middleware

import (
	"testing"
)

func TestExtract(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"canglimo", "canglimo"},
		{"canglimo.atom", "canglimo"},
		{"canglimo.atom/rss", "canglimo"},
		{"canglimo.atom/feed", "canglimo"},
		{"canglimo.com", "canglimo"},
		{"canglimo/rss.com", "canglimo"},
		{"canglimo/feed.com", "canglimo"},
	}
	for _, c := range cases {
		got := extractFeedID(c.in)
		if got != c.want {
			t.Errorf("Extract(%q) == %q, want %q", c.in, got, c.want)
		}
	}
}
