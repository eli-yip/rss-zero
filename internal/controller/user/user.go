package user

import (
	"net/http"

	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/labstack/echo/v5"
)

type UserResponse struct {
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}

func GetUserInfo(c *echo.Context) (err error) {
	username, err := echo.ContextGet[string](c, "username")
	if err != nil {
		return httputil.NewHTTPError(http.StatusInternalServerError, "missing username")
	}
	nickname, err := echo.ContextGet[string](c, "nickname")
	if err != nil {
		return httputil.NewHTTPError(http.StatusInternalServerError, "missing nickname")
	}
	return c.JSON(http.StatusOK, httputil.NewResp("found user", &UserResponse{
		Username: username,
		Nickname: nickname,
	}))
}
