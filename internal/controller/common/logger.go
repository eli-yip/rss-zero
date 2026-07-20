package common

import (
	"github.com/labstack/echo/v5"
	"go.uber.org/zap"
)

func ExtractLogger(c *echo.Context) *zap.Logger {
	logger, err := echo.ContextGet[*zap.Logger](c, "logger")
	if err != nil || logger == nil {
		return zap.NewNop()
	}
	return logger
}
