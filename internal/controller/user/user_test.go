package user

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/eli-yip/rss-zero/config"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

func TestUserAdminFlag(t *testing.T) {
	original := config.C.Settings.Debug
	t.Cleanup(func() { config.C.Settings.Debug = original })
	for _, tc := range []struct {
		name, groups string
		debug, want  bool
	}{
		{name: "regular", groups: "users"},
		{name: "missing groups"},
		{name: "administrator", groups: "users,lldap_admin", want: true},
		{name: "similar group", groups: "not_lldap_admin"},
		{name: "debug", debug: true, want: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config.C.Settings.Debug = tc.debug
			e := echo.New()
			e.GET("/user", func(c *echo.Context) error {
				c.Set("username", "reader")
				c.Set("nickname", "Reader")
				return GetUserInfo(c)
			})
			req := httptest.NewRequest(http.MethodGet, "/user", nil)
			req.Header.Set("Remote-Groups", tc.groups)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			require.Equal(t, http.StatusOK, rec.Code)
			var body struct {
				Data UserResponse `json:"data"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			require.Equal(t, tc.want, body.Data.IsAdmin)
			require.Equal(t, "reader", body.Data.Username)
		})
	}
}
