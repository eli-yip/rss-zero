package request

import (
	"net/http"
)

type Requester interface {
	// WithLimiter makes request to the given url with limiter,
	// and it will parse the response body and check if it is valid api response
	WithLimiter(string) ([]byte, error)
	// WithLimiterRawData makes request to the given url with limiter and returns raw data,
	// commonly used for downloading file
	WithLimiterRawData(string) ([]byte, error)
	// WithLimiterStream makes request to the given url with limiter and returns http response,
	WithLimiterStream(string) (*http.Response, error)
	// WithoutLimiter makes request to the given url without limiter,
	// and it will parse the response body and check if it is valid api response
	WithoutLimiter(string) ([]byte, error)
}
