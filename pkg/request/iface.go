package request

import (
	"net/http"
)

type RequestIface interface {
	SendWithLimiter(string) ([]byte, error)
	SendWithLimiterStream(string) (*http.Response, error)
	SendWithoutLimiter(string) ([]byte, error)
}
