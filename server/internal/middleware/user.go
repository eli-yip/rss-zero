package middleware

import (
	"net/http"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/labstack/echo/v4"
)

func InjectUser() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			username := c.Request().Header.Get("Remote-User")
			nickname := c.Request().Header.Get("Remote-Name")

			if config.C.Settings.Debug {
				username, nickname = "jason", "Jason"
			}
			if username == "" || nickname == "" {
				return c.JSON(http.StatusBadRequest, common.WrapResp("missing username or nickname"))
			}
			c.Set("username", username)
			c.Set("nickname", nickname)
			return next(c)
		}
	}
}
