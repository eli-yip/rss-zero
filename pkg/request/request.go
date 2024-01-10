package request

import (
	"net/http"
)

type Requester interface {
	WithLimiter(string) ([]byte, error)
	WithLimiterStream(string) (*http.Response, error)
	WithoutLimiter(string) ([]byte, error)
}
