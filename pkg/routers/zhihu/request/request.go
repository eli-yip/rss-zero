package request

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/eli-yip/rss-zero/config"
	"go.uber.org/zap"
)

type Requester interface {
	// LimitRaw requests to the given url with limiter and returns raw data,
	LimitRaw(string) ([]byte, error)
	// NoLimitRaw requests to the given url without limiter and returns raw data,
	// Commonly used in file download
	NoLimitStream(string) (*http.Response, error)
}

var (
	ErrBadResponse   = errors.New("bad response")
	ErrInvalidCookie = errors.New("invalid cookie")
	ErrMaxRetry      = errors.New("max retry")
	ErrUnreachable   = errors.New("unreachable")
	ErrGetXZSE96     = errors.New("fail to get x-zse-96")
	ErrNewRequest    = errors.New("fail to new a request")
	ErrNeedLogin     = errors.New("need login")
)

const (
	userAgent    = "ZhihuHybrid com.zhihu.android/Futureve/6.59.0 Mozilla/5.0 (Linux; Android 10; GM1900 Build/QKQ1.190716.003; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/85.0.4183.127 Mobile Safari/537.36"
	apiUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Safari/537.36 Edg/87.0.664.75"
)

type RequestService struct {
	client   *http.Client
	limiter  chan struct{}
	maxRetry int // default 5
	log      *zap.Logger
}

func NewRequestService(logger *zap.Logger) (Requester, error) {
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
			time.Sleep(time.Duration(30+rand.IntN(6)) * time.Second)
		}
	}()

	return s, nil
}

type Error403 struct {
	Error struct {
		NeedLogin bool `json:"need_login"`
	} `json:"error"`
}

type URLRequest struct {
	URL string `json:"url"`
}

// Send request with limiter, used for all zhihu api requests
func (r *RequestService) LimitRaw(u string) (respByte []byte, err error) {
	logger := r.log.With(zap.String("url", u))
	logger.Info("request with limiter for raw data")

	for i := 0; i < r.maxRetry; i++ {
		logger := logger.With(zap.Int("index", i))
		<-r.limiter

		var reqBody URLRequest
		reqBody.URL = u
		reqBodyByte, err := json.Marshal(reqBody)
		if err != nil {
			logger.Error("fail to marshal request body", zap.Error(err))
			continue
		}
		resp, err := http.Post(config.C.ZhihuEncryptionURL, "application/json", bytes.NewBuffer(reqBodyByte))
		if err != nil {
			logger.Error("fail to request url", zap.Error(err))
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bytes, _ := io.ReadAll(resp.Body)
			if resp.StatusCode == http.StatusForbidden {
				var e403 Error403
				if err = json.Unmarshal(bytes, &e403); err != nil {
					logger.Error("fail to unmarshal 403 error", zap.Error(err))
					continue
				}
				if e403.Error.NeedLogin {
					logger.Error("need login")
					return nil, ErrNeedLogin
				}
			}
			if resp.StatusCode == http.StatusNotFound {
				logger.Error("404 not found")
				return nil, ErrUnreachable
			}
			logger.Error("status code error", zap.Int("status_code", resp.StatusCode))
			continue
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Error("fail to read response body", zap.Error(err))
			continue
		}
		return body, nil
	}

	logger.Error("fail to get zhihu response with limit", zap.Error(err))
	return nil, err
}

func (r *RequestService) NoLimitStream(u string) (resp *http.Response, err error) {
	logger := r.log.With(zap.String("url", u))
	logger.Info("request without limit for stream")

	for i := 0; i < r.maxRetry; i++ {
		logger := logger.With(zap.Int("index", i))

		var req *http.Request
		req, err = r.setReq(u)
		if err != nil {
			logger.Error("fail to new a request", zap.Error(err))
			continue
		}

		resp, err = r.client.Do(req)
		if err != nil {
			logger.Error("fail to request url", zap.Error(err))
			continue
		}
		// do not defer resp body close here because we will save it to minio

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			err = fmt.Errorf("bad response status code: %d", resp.StatusCode)
			logger.Error("bad status code", zap.Int("status code", resp.StatusCode))
			continue
		}

		return resp, nil
	}

	logger.Error("fail to get zhihu no limit stream", zap.Error(err))
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
