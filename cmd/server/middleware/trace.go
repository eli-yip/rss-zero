package middleware

import (
	"strings"
	"time"

	"github.com/kataras/iris/v12"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func LogRequest(logger *zap.Logger) iris.Handler {
	return func(ctx iris.Context) {
		startTime := time.Now()
		method := ctx.Method()
		path := ctx.Path()
		ip := getClientIP(ctx)
		requestID := ctx.GetID().(string)
		message := "Received request"
		logger.Info(message,
			zap.String("request_id", requestID),
			zap.String("method", method),
			zap.String("path", path),
			zap.String("ip", ip),
		)

		ctx.Next()

		endTime := time.Now()
		latency := endTime.Sub(startTime)
		statusCode := ctx.GetStatusCode()
		var level zapcore.Level
		if statusCode >= iris.StatusInternalServerError {
			level = zapcore.ErrorLevel
		} else if statusCode >= iris.StatusBadRequest {
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
	}
}

func getClientIP(c iris.Context) string {
	clientIP := c.GetHeader("X-Forwarded-For")

	if clientIP == "" {
		clientIP = c.RemoteAddr()
	}

	parts := strings.Split(clientIP, ",")
	if len(parts) > 0 {
		return strings.TrimSpace(parts[0])
	}

	return clientIP
}
