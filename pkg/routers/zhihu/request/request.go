package request

import (
	"errors"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/encrypt"
	"go.uber.org/zap"
)

var (
	ErrBadResponse   = errors.New("bad response")
	ErrInvalidCookie = errors.New("invalid cookie")
	ErrMaxRetry      = errors.New("max retry")
	ErrUnreachable   = errors.New("unreachable")
)

const userAgent = "ZhihuHybrid com.zhihu.android/Futureve/6.59.0 Mozilla/5.0 (Linux; Android 10; GM1900 Build/QKQ1.190716.003; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/85.0.4183.127 Mobile Safari/537.36"

type RequestService struct {
	client   *http.Client
	limiter  chan struct{}
	maxRetry int
	dC0      string
	cookies  string
	log      *zap.Logger
}

func NewRequestService(logger *zap.Logger) *RequestService {
	const defaultMaxRetry = 5
	s := &RequestService{
		client:   &http.Client{},
		limiter:  make(chan struct{}),
		maxRetry: defaultMaxRetry,
		log:      logger,
	}

	cookies, err := encrypt.GetCookies()
	if err != nil {
		// TODO: Add error check for this
		logger.Fatal("fail to get cookies", zap.Error(err))
	}
	found := false
	for _, c := range cookies {
		if c.Name == "d_c0" {
			logger.Info("get d_c0 cookie", zap.String("value", c.Value))
			s.dC0 = c.Value
			found = true
		}
	}
	if !found {
		// TODO: Add error check for this
		logger.Fatal("fail to find d_c0 cookie")
	}

	cookieStr := encrypt.CookiesToString(cookies)
	s.cookies = cookieStr

	go func() {
		for {
			s.limiter <- struct{}{}
			time.Sleep(time.Duration(3+rand.Intn(6)) * time.Second)
		}
	}()

	return s
}

// Send request with limiter with error check
// Now it's only used with api.zhihu.com v4 answer api
func (r *RequestService) Limit(u string) (respByte []byte, err error) {
	logger := r.log.With(zap.String("url", u))

	logger.Info("start to get zhihu API response with limit")
	for i := 0; i < r.maxRetry; i++ {
		logger := logger.With(zap.Int("index", i))
		<-r.limiter

		req, err := r.setReq(u)
		if err != nil {
			logger.Error("fail to new a request", zap.Error(err))
			continue
		}

		resp, err := r.client.Do(req)
		if err != nil || (resp.StatusCode != http.StatusOK &&
			resp.StatusCode != http.StatusMethodNotAllowed &&
			resp.StatusCode != http.StatusNotFound) {
			if resp != nil && resp.Body != nil {
				// Close response body when error.
				resp.Body.Close()
			}
			logger.Error("fail to request url", zap.Error(err))
			continue
		}

		bytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			logger.Error("fail to read response body", zap.Error(err))
			continue
		}

		if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotFound {
			// NOTE: this error will be logged in the caller
			return nil, ErrUnreachable
		}
		return bytes, nil
	}

	if err == nil {
		err = ErrMaxRetry
	}
	logger.Error("fail to get zhihu API response with limit", zap.Error(err))
	return nil, err
}

// Send request with limiter, used for zhihu
// Now it's only used with api.zhihu.com answers api
func (r *RequestService) LimitRaw(u string) (respByte []byte, err error) {
	logger := r.log.With(zap.String("url", u))
	logger.Info("request with limiter for raw data")

	for i := 0; i < r.maxRetry; i++ {
		logger := logger.With(zap.Int("index", i))
		<-r.limiter

		req, err := r.setReq(u)
		if err != nil {
			logger.Error("fail to new a request", zap.Error(err))
			continue
		}

		xzse96, err := encrypt.GetXZSE96(u, r.dC0)
		if err != nil {
			logger.Error("fail to get xzse96", zap.Error(err))
			continue
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Safari/537.36 Edg/87.0.664.75")
		req.Header.Set("x-zse-93", "101_3_3.0")
		req.Header.Set("x-zse-96", xzse96)
		req.Header.Set("cookie", r.cookies)

		resp, err := r.client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
			logger.Error("fail to request url", zap.Error(err))
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			logger.Error("fail to read response body", zap.Error(err))
			continue
		}
		return body, nil
	}

	if err == nil {
		err = ErrMaxRetry
	}
	logger.Error("fail to get zhihu response with limit", zap.Error(err))
	return nil, err
}

// Send request with limiter and get a stream result, not used
func (r *RequestService) LimitStream(u string) (resp *http.Response, err error) {
	return nil, nil
}

// Send request without limiter, not used
func (r *RequestService) NoLimit(u string) (respByte []byte, err error) {
	return nil, nil
}

// Zsxq api does not support no limit stream
func (r *RequestService) NoLimitStream(u string) (resp *http.Response, err error) {
	logger := r.log.With(zap.String("url", u))
	logger.Info("request without limit for stream")

	for i := 0; i < r.maxRetry; i++ {
		logger := logger.With(zap.Int("index", i))

		req, err := r.setReq(u)
		if err != nil {
			logger.Error("fail to new a request", zap.Error(err))
			continue
		}

		resp, err = r.client.Do(req)
		// When request failed or status code is not 200, error.
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
			logger.Error("fail to request url", zap.Error(err))
			continue
		}

		return resp, nil
	}

	if err == nil {
		err = ErrMaxRetry
	}
	logger.Error("fail to get zsxq API response with limit", zap.Error(err))
	return nil, err
}

// Set request header and method
func (r *RequestService) setReq(u string) (req *http.Request, err error) {
	req, err = http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	return req, nil
}
