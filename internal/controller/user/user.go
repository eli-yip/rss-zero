package user

import (
	"net/http"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/labstack/echo/v4"
)

type UserResponse struct {
	Username string `json:"username"`
	Nickname string `json:"nickname"`
}

func GetUserInfo(c echo.Context) (err error) {
	return c.JSON(http.StatusOK, common.WrapRespWithData("found user", &UserResponse{
		Username: c.Get("username").(string),
		Nickname: c.Get("nickname").(string),
	}))
}
