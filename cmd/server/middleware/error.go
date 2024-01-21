package middleware

import (
	"github.com/kataras/iris/v12"
	"go.uber.org/zap"
)

type HttpError struct {
	Code    int
	Message string
}

func ErrorHandler(logger *zap.Logger) iris.Handler {
	return func(ctx iris.Context) {
		ctx.Next()

		if err := ctx.Values().Get("error"); err != nil {
			httpErr, ok := err.(*HttpError)
			if !ok {
				ctx.StatusCode(iris.StatusInternalServerError)
				return
			}
			ctx.StatusCode(httpErr.Code)
			_, _ = ctx.Text(httpErr.Message)
		}
	}
}
