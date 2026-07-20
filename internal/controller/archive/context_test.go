package archive

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"

	"github.com/eli-yip/rss-zero/pkg/httputil"
)

func TestContextUsername(t *testing.T) {
	tests := []struct {
		name      string
		setValue  bool
		value     any
		want      string
		wantError bool
	}{
		{name: "字符串值", setValue: true, value: "alice", want: "alice"},
		{name: "缺少值", wantError: true},
		{name: "类型错误", setValue: true, value: 42, wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			c := e.NewContext(httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder())
			if tt.setValue {
				c.Set("username", tt.value)
			}

			username, err := contextUsername(c)
			if !tt.wantError {
				require.NoError(t, err)
				require.Equal(t, tt.want, username)
				return
			}

			var responseError *httputil.ResponseError
			require.ErrorAs(t, err, &responseError)
			require.Equal(t, http.StatusInternalServerError, responseError.Code)
		})
	}
}
