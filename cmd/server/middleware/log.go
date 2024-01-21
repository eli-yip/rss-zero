package middleware

import (
	"github.com/kataras/iris/v12"
	"go.uber.org/zap"
)

func LoggerMiddleware(logger *zap.Logger) iris.Handler {
	return func(ctx iris.Context) {
		requestID := ctx.GetID().(string)
		loggerWithID := logger.With(zap.String("request_id", requestID))
		ctx.Values().Set("logger", loggerWithID)
		ctx.Next()
	}
}
