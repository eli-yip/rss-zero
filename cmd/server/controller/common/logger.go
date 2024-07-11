package common

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func ExtractLogger(c echo.Context) *zap.Logger {
	return c.Get("logger").(*zap.Logger)
}
