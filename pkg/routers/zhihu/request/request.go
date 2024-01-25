package request

import (
	"errors"
	"io"
	"math/rand"
	"net/http"
	"time"

	"go.uber.org/zap"
)

var (
	ErrBadResponse   = errors.New("bad response")
	ErrInvalidCookie = errors.New("invalid cookie")
	ErrMaxRetry      = errors.New("max retry")
)

const userAgent = "ZhihuHybrid com.zhihu.android/Futureve/6.59.0 Mozilla/5.0 (Linux; Android 10; GM1900 Build/QKQ1.190716.003; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/85.0.4183.127 Mobile Safari/537.36"

type RequestService struct {
	client   *http.Client
	limiter  chan struct{}
	maxRetry int
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

	go func() {
		for {
			s.limiter <- struct{}{}
			time.Sleep(time.Duration(7+rand.Intn(6)) * time.Second)
		}
	}()

	return s
}

// Send request with limiter with error check, not used
func (r *RequestService) Limit(u string) (respByte []byte, err error) {
	return nil, nil
}

// Send request with limiter, used for zhihu
func (r *RequestService) LimitRaw(u string) (respByte []byte, err error) {
	logger := r.log.With(zap.String("url", u))
	logger.Info("request with limiter for raw data", zap.String("url", u))

	for i := 0; i < r.maxRetry; i++ {
		logger := logger.With(zap.Int("index", i))
		<-r.limiter

		req, err := r.setReq(u)
		if err != nil {
			logger.Error("fail to new a request", zap.Error(err))
			continue
		}

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
