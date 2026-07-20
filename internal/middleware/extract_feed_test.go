package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
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

func TestExtractFeedIDMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		pathValues echo.PathValues
		want       string
	}{
		{name: "带 feed 参数", pathValues: echo.PathValues{{Name: "feed", Value: "canglimo.atom"}}, want: "canglimo"},
		{name: "无 feed 参数", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			c := e.NewContext(httptest.NewRequest(http.MethodGet, "/rss", nil), httptest.NewRecorder())
			if tt.pathValues != nil {
				c.SetPathValues(tt.pathValues)
			}

			err := ExtractFeedID()(func(c *echo.Context) error {
				feedID, err := echo.ContextGet[string](c, "feed_id")
				require.NoError(t, err)
				require.Equal(t, tt.want, feedID)
				return nil
			})(c)
			require.NoError(t, err)
		})
	}
}
