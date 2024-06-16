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

	"github.com/rs/xid"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
)

type Requester interface {
	// LimitRaw requests to the given url with limiter and returns raw data,
	LimitRaw(string, *zap.Logger) ([]byte, error)
	// NoLimitRaw requests to the given url without limiter and returns raw data,
	// Commonly used in file download
	NoLimitStream(string) (*http.Response, error)
	// Clear d_c0 cache
	ClearCache(*zap.Logger)
}

var (
	ErrBadResponse = errors.New("bad response")
	ErrEmptyDC0    = errors.New("empty d_c0 cookie")
	ErrMaxRetry    = errors.New("max retry")
	ErrUnreachable = errors.New("unreachable")
	ErrNeedLogin   = errors.New("need login")
)

const (
	userAgent    = "ZhihuHybrid com.zhihu.android/Futureve/6.59.0 Mozilla/5.0 (Linux; Android 10; GM1900 Build/QKQ1.190716.003; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/85.0.4183.127 Mobile Safari/537.36"
	apiUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Safari/537.36 Edg/87.0.664.75"
)

type RequestService struct {
	client   *http.Client
	limiter  chan struct{}
	maxRetry int // default 5
	logger   *zap.Logger
}

func NewRequestService(logger *zap.Logger) (Requester, error) {
	const defaultMaxRetry = 5
	s := &RequestService{
		client:   &http.Client{},
		limiter:  make(chan struct{}),
		maxRetry: defaultMaxRetry,
		logger:   logger,
	}

	go func() {
		for {
			s.limiter <- struct{}{}
			time.Sleep(time.Duration(30+rand.IntN(30)) * time.Second)
		}
	}()

	return s, nil
}

type Error403 struct {
	Error struct {
		NeedLogin bool `json:"need_login"`
	} `json:"error"`
}

type EncryptReq struct {
	RequestID string `json:"request_id"`
	URL       string `json:"url"`
}

type EncryptErrResp struct {
	Error string `json:"error"`
}

// Send request with limiter, used for all zhihu api requests
func (r *RequestService) LimitRaw(u string, logger *zap.Logger) (respByte []byte, err error) {
	requestTaskID := xid.New().String()
	logger.Info("Start to get zhihu raw data with limit, waiting for limiter", zap.String("url", u), zap.String("request_task_id", requestTaskID))

	for i := 0; i < r.maxRetry; i++ {
		currentRequestTaskID := fmt.Sprintf("%s_%d", requestTaskID, i)
		logger := logger.With(zap.String("request_task_id", currentRequestTaskID))
		<-r.limiter
		logger.Info("Get limiter successfully, start to request url")

		reqBodyByte, err := json.Marshal(EncryptReq{RequestID: currentRequestTaskID, URL: u})
		if err != nil {
			logger.Error("Failed to marshal request body", zap.Error(err))
			continue
		}

		resp, err := http.Post(config.C.Utils.ZhihuEncryptionURL+"/data", "application/json", bytes.NewBuffer(reqBodyByte))
		if err != nil {
			logger.Error("Failed to request", zap.Error(err))
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			logger.Error("Failed to read response", zap.Error(err))
			continue
		}

		switch resp.StatusCode {
		case http.StatusOK:
			logger.Info("Get zhihu raw data successfully")
			return body, nil
		case http.StatusForbidden:
			var e403 Error403
			if err = json.Unmarshal(body, &e403); err != nil {
				logger.Error("Failed to unmarshal 403 error", zap.Error(err))
				continue
			}
			if e403.Error.NeedLogin {
				logger.Error("Need login according to 403 error")
				return nil, ErrNeedLogin
			}
		case http.StatusNotFound:
			logger.Error("404 error")
			return nil, ErrUnreachable
		case http.StatusInternalServerError:
			logger.Error("Failed to get d_c0 cookie")
			return nil, ErrEmptyDC0
		case http.StatusNotImplemented:
			var encryptErrResp EncryptErrResp
			if err = json.Unmarshal(body, &encryptErrResp); err != nil {
				logger.Error("Failed to unmarshal 501 error", zap.Error(err))
				continue
			}
			logger.Error("501 error", zap.String("error", encryptErrResp.Error))
			return nil, ErrBadResponse
		default:
			logger.Error("Bad status code", zap.Int("status_code", resp.StatusCode))
			continue
		}
	}

	r.logger.Error("Failed to get zhihu raw data", zap.Error(err), zap.String("request_task_id", requestTaskID))
	return nil, err
}

func (r *RequestService) NoLimitStream(u string) (resp *http.Response, err error) {
	logger := r.logger.With(zap.String("url", u))
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

func (rs *RequestService) ClearCache(logger *zap.Logger) {
	logger.Info("Start to clear d_c0 cache")
	_, err := http.Post(config.C.Utils.ZhihuEncryptionURL+"/clear-cache", "application/json", nil)
	if err != nil {
		logger.Error("failed to clear d_c0 cache", zap.Error(err))
		return
	}
	logger.Info("Clear d_c0 cache successfully")
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
