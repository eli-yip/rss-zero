package request

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/net/publicsuffix"

	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/request"
)

var (
	ErrBadResponse   = errors.New("bad response")
	ErrInvalidCookie = errors.New("invalid cookie")
	ErrMaxRetry      = errors.New("max retry")
)

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

type RequestService struct {
	client       *http.Client
	emptyClient  *http.Client
	limiter      chan struct{}
	maxRetry     int
	redisService redis.Redis
	logger       *zap.Logger
}

func NewRequestService(cookie string, redisService redis.Redis,
	logger *zap.Logger) request.Requester {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

	const defaultMaxRetry = 5
	s := &RequestService{
		client:       &http.Client{Jar: jar},
		emptyClient:  &http.Client{},
		limiter:      make(chan struct{}),
		maxRetry:     defaultMaxRetry,
		redisService: redisService,
		logger:       logger,
	}

	s.SetCookies(cookie)

	go func() {
		for {
			s.limiter <- struct{}{}
			time.Sleep(time.Duration(30+rand.IntN(6)) * time.Second)
		}
	}()

	return s
}

func (r *RequestService) SetCookies(c string) {
	var domains []string = []string{
		"articles.zsxq.com",
		"api.zsxq.com",
	}

	for _, d := range domains {
		u, _ := url.Parse("https://" + d)
		// split cookies by ";" into cookie parts
		for _, cp := range strings.SplitN(c, ";", -1) {
			if n, v, ok := strings.Cut(strings.TrimSpace(cp), "="); ok {
				cookie := &http.Cookie{Name: n, Value: v}
				r.client.Jar.SetCookies(u, []*http.Cookie{cookie})
			}
		}
		r.logger.Info("set cookie successfully",
			zap.String("cookie", c), zap.String("domain", d))
	}
}

// apiResp is the typical response of zsxq api
type apiResp struct {
	Succeeded bool `json:"succeeded"`
}

// badAPIResp zsxq api bad response
type badAPIResp struct {
	// - 1059 for too many requests due to no sign
	//
	// - 401 for invalid cookie
	Code int `json:"code"`
}

// Send request with limiter, used for zsxq api.
func (r *RequestService) Limit(u string) (respByte []byte, err error) {
	logger := r.logger.With(zap.String("url", u))

	logger.Info("start to get zsxq API response with limit")
	for i := 0; i < r.maxRetry; i++ {
		logger := logger.With(zap.Int("index", i))
		<-r.limiter // block until get a token

		var req *http.Request
		if req, err = r.setReq(u); err != nil {
			logger.Error("fail to new a request", zap.Error(err))
			continue
		}

		var resp *http.Response
		if resp, err = r.client.Do(req); err != nil {
			logger.Error("fail to request url", zap.Error(err))
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("bad response status code: %d", resp.StatusCode)
			logger.Error("Bad response status code", zap.Error(err))
			continue
		}

		var bytes []byte
		if bytes, err = io.ReadAll(resp.Body); err != nil {
			logger.Error("fail to read response body", zap.Error(err))
			continue
		}

		var respData apiResp
		if err = json.Unmarshal(bytes, &respData); err != nil {
			logger.Error("fail to unmarshal response", zap.Error(err))
			continue
		}

		if respData.Succeeded {
			return bytes, nil
		}

		var badResp badAPIResp
		if err = json.Unmarshal(bytes, &badResp); err != nil {
			logger.Error("fail to unmarshal bad resp", zap.Error(err), zap.Any("badResp", badResp))
			continue
		}
		switch badResp.Code {
		case 401:
			logger.Error("invalid zsxq cookie, clear cookie")
			if err = r.redisService.Del(redis.ZsxqCookiePath); err != nil {
				logger.Error("fail to delete zsxq cookie", zap.Error(err))
			}
			return nil, ErrInvalidCookie
		case 1059:
			logger.Error("too many requests due to no sign, sleep 10s")
			time.Sleep(time.Second * 10)
			continue
		default:
			logger.Error("unknown bad response", zap.Int("status_code", badResp.Code))
			continue
		}
	}

	if err == nil {
		err = ErrMaxRetry
	}
	logger.Error("fail to get zsxq API response with limit", zap.Error(err))
	return nil, err
}

// Send request with limiter, used for zsxq article
func (r *RequestService) LimitRaw(u string) (respByte []byte, err error) {
	logger := r.logger.With(zap.String("url", u))
	logger.Info("request with limiter for raw data", zap.String("url", u))

	for i := 0; i < r.maxRetry; i++ {
		logger := logger.With(zap.Int("index", i))
		<-r.limiter

		var req *http.Request
		if req, err = r.setReq(u); err != nil {
			logger.Error("fail to new a request", zap.Error(err))
			continue
		}

		var resp *http.Response
		if resp, err = r.client.Do(req); err != nil {
			logger.Error("fail to request url", zap.Error(err))
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("bad response status code: %d", resp.StatusCode)
			logger.Error("Bad response status code", zap.Error(err))
			continue
		}

		var body []byte
		if body, err = io.ReadAll(resp.Body); err != nil {
			logger.Error("fail to read response body", zap.Error(err))
			continue
		}
		return body, nil
	}

	if err == nil {
		err = ErrMaxRetry
	}
	logger.Error("fail to get zsxq API response with limit", zap.Error(err))
	return nil, err
}

// Send request with limiter and get a stream result,
// used for zsxq voices
func (r *RequestService) LimitStream(u string) (resp *http.Response, err error) {
	logger := r.logger.With(zap.String("url", u))
	logger.Info("request with limiter for stream")

	for i := 0; i < r.maxRetry; i++ {
		logger := logger.With(zap.Int("index", i))

		var req *http.Request
		if req, err = r.setReq(u); err != nil {
			logger.Error("fail to new a request", zap.Error(err))
			continue
		}

		var resp *http.Response
		if resp, err = r.client.Do(req); err != nil {
			logger.Error("fail to request url", zap.Error(err))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			err = fmt.Errorf("bad response status code: %d", resp.StatusCode)
			logger.Error("Bad response status code", zap.Error(err))
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

// Send request without limiter, used for zsxq cdn
func (r *RequestService) NoLimit(u string) (respByte []byte, err error) {
	logger := r.logger.With(zap.String("url", u))
	logger.Info("without limiter", zap.String("url", u))
	for i := 0; i < r.maxRetry; i++ {
		logger := logger.With(zap.Int("index", i))

		var req *http.Request
		if req, err = r.setReq(u); err != nil {
			logger.Error("fail to new a request", zap.Error(err))
			continue
		}

		var resp *http.Response
		if resp, err = r.client.Do(req); err != nil {
			logger.Error("fail to request url", zap.Error(err))
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("bad response status code: %d", resp.StatusCode)
			logger.Error("Bad response status code", zap.Error(err))
			continue
		}

		var bytes []byte
		if bytes, err = io.ReadAll(resp.Body); err == nil {
			return bytes, nil
		}
	}

	if err == nil {
		err = ErrMaxRetry
	}
	logger.Error("fail to get zsxq API response without limit", zap.Error(err))
	return nil, err
}

// Set request header and method
func (r *RequestService) setReq(u string) (req *http.Request, err error) {
	req, err = http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://wx.zsxq.com/")
	return req, nil
}

// Zsxq api does not support no limit stream
func (r *RequestService) NoLimitStream(u string) (resp *http.Response, err error) {
	return nil, errors.New("NoLimitStream() should not be called in zsxq reqeust")
}
