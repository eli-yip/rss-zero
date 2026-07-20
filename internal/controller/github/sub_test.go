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
			query: "?sub_id=sub-1&sub_id=&prerelease=true&deleted=false",
			want:  filterConfig{SubID: []string{"sub-1"}, Prerelease: true, Deleted: true},
		},
		{
			name:  "prerelease 仅接受 true",
			query: "?prerelease=1",
			want:  filterConfig{},
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
