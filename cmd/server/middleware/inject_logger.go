package middleware

import (
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func InjectLogger(logger *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			reqeustID := c.Response().Header().Get(echo.HeaderXRequestID)
			logger := logger.With(zap.String("request_id", reqeustID))
			c.Set("logger", logger)
			return next(c)
		}
	}
}
