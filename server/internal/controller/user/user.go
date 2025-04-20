package user

import (
	"net/http"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type UserResponse struct {
	Username string `json:"username"`
}

func GetUserInfo(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	logger.Info("Receive get user info")

	user := c.Request().Header.Get("Remote-User")
	if config.C.Settings.Debug {
		user = "jason"
	}
	if user == "" {
		logger.Info("User not found")
		return c.JSON(http.StatusBadRequest, common.WrapResp("user not found"))
	}
	logger.Info("User found: ", zap.String("user", user))

	return c.JSON(http.StatusOK, common.WrapRespWithData("found user", &UserResponse{Username: user}))
}
