// Package httputil provides the unified HTTP response envelope used across all
// JSON (/api/v1) endpoints: success responses are {message, data} and error
// responses are {message}, the latter produced centrally by NewHTTPErrorHandler.
package httputil

// Resp is the success envelope. Fields are always present (no omitempty) so the
// frontend can unwrap data unconditionally.
type Resp[T any] struct {
	Message string `json:"message"`
	Data    T      `json:"data"`
}

// EmptyResp is the message-only envelope (no data payload).
type EmptyResp struct {
	Message string `json:"message"`
}

func NewResp[T any](message string, data T) Resp[T] {
	return Resp[T]{Message: message, Data: data}
}

func NewMessage(message string) EmptyResp { return EmptyResp{Message: message} }
