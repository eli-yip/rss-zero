package middleware

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func LogRequest(logger *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			startTime := time.Now()
			req := c.Request()
			method := req.Method
			path := req.URL.Path
			ip := c.RealIP()
			requestID := c.Response().Header().Get(echo.HeaderXRequestID)
			message := "Received request"
			logger.Info(message,
				zap.String("request_id", requestID),
				zap.String("method", method),
				zap.String("path", path),
				zap.String("ip", ip),
			)

			if err := next(c); err != nil {
				c.Error(err)
			}

			endTime := time.Now()
			latency := endTime.Sub(startTime)
			statusCode := c.Response().Status
			var level zapcore.Level
			if statusCode >= http.StatusInternalServerError {
				level = zapcore.ErrorLevel
			} else if statusCode >= http.StatusBadRequest {
				level = zapcore.WarnLevel
			} else {
				level = zapcore.InfoLevel
			}
			message = "Sent response"
			logger.Check(level, message).Write(
				zap.String("request_id", requestID),
				zap.Duration("latency", latency),
				zap.Int("status_code", statusCode),
			)

			return nil
		}
	}
}
