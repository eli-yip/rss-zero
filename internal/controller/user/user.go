package user

import (
	"net/http"

	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/labstack/echo/v4"
)

type UserResponse struct {
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}

func GetUserInfo(c echo.Context) (err error) {
	return c.JSON(http.StatusOK, httputil.NewResp("found user", &UserResponse{
		Username: c.Get("username").(string),
		Nickname: c.Get("nickname").(string),
	}))
}
