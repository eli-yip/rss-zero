package httputil

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"
)

// ResponseError is an error carrying an HTTP status code. Handlers return it
// (via NewHTTPError) instead of writing error responses themselves; the
// centralized handler renders it as {message} with the given code.
type ResponseError struct {
	Code    int
	Message string
}

func (e *ResponseError) Error() string { return e.Message }

func NewHTTPError(code int, message string) *ResponseError {
	return &ResponseError{Code: code, Message: message}
}

// NewHTTPErrorHandler 返回统一渲染 {message} 响应的 Echo 错误处理器。
// 请求级 logger 不可用时，baseLogger 作为兜底。
func NewHTTPErrorHandler(baseLogger *zap.Logger) echo.HTTPErrorHandler {
	return func(c *echo.Context, err error) {
		response, unwrapErr := echo.UnwrapResponse(c.Response())
		if unwrapErr == nil && response.Committed {
			return
		}

		logger := baseLogger
		if l, getErr := echo.ContextGet[*zap.Logger](c, "logger"); getErr == nil && l != nil {
			logger = l
		}

		code := http.StatusInternalServerError
		message := http.StatusText(code)

		var re *ResponseError
		var he *echo.HTTPError
		switch {
		case errors.As(err, &re):
			code, message = re.Code, re.Message
		case errors.As(err, &he):
			// echo-originated errors (404/405, binding failures): use the
			// status text, never echo's internal message, to avoid leaks.
			code, message = he.Code, http.StatusText(he.Code)
		}

		if logger != nil {
			logger.Error("handle request error", zap.Int("code", code), zap.Error(err))
		}

		if jsonErr := c.JSON(code, NewMessage(message)); jsonErr != nil && logger != nil {
			logger.Error("failed to write error response", zap.Error(jsonErr))
		}
	}
}
