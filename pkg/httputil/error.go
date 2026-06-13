package httputil

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
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

// NewHTTPErrorHandler returns an echo v4 HTTPErrorHandler that renders every
// error as the unified {message} envelope. baseLogger is the fallback when the
// request-scoped logger (set by middleware.InjectLogger) is unavailable.
func NewHTTPErrorHandler(baseLogger *zap.Logger) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}

		logger := baseLogger
		if l, ok := c.Get("logger").(*zap.Logger); ok && l != nil {
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
