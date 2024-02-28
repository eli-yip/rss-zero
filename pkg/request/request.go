package request

import "net/http"

type Requester interface {
	// Limit requests to the given url with limiter and returns data,
	// and it will validate the response json data
	Limit(string) ([]byte, error)
	// LimitRaw requests to the given url with limiter and returns raw data,
	LimitRaw(string) ([]byte, error)
	// LimitStream requests to the given url with limiter and returns http response,
	// Commonly used in file download
	LimitStream(string) (*http.Response, error)
	// NoLimit requests to the given url without limiter
	NoLimit(string) ([]byte, error)
	// NoLimitRaw requests to the given url without limiter and returns raw data,
	// Commonly used in file download
	NoLimitStream(string) (*http.Response, error)
}
