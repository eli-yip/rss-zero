package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

func TestParseFilterConfig(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  filterConfig
	}{
		{name: "无筛选", want: filterConfig{}},
		{
			name:  "多值与存在标记",
			query: "?author=alice&author=&sub_id=sub-1&type=answer&type=pin&deleted=",
			want: filterConfig{
				AuthorID:    []string{"alice"},
				SubID:       []string{"sub-1"},
				ContentType: []string{"answer", "pin"},
				Deleted:     true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			c := e.NewContext(httptest.NewRequest(http.MethodGet, "/subscriptions"+tt.query, nil), httptest.NewRecorder())

			got, err := parseFilterConfig(c)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
