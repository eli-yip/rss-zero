package request

import (
	"net/http"
)

type Requester interface {
	// Limit requests to the given url with limiter,
	// and it will parse the response body and check if it is valid api response
	Limit(string) ([]byte, error)
	// LimitRaw requests to the given url with limiter and returns raw data,
	// commonly used for getting zsxq articles
	LimitRaw(string) ([]byte, error)
	// LimitStream requests to the given url with limiter and returns http response,
	LimitStream(string) (*http.Response, error)
	// NoLimit requests to the given url without limiter
	NoLimit(string) ([]byte, error)
}
