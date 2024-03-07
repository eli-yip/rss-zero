package request

import (
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/request"
	"go.uber.org/zap"
	"golang.org/x/net/publicsuffix"
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
	log          *zap.Logger
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
		log:          logger,
	}

	s.SetCookies(cookie)

	go func() {
		for {
			s.limiter <- struct{}{}
			time.Sleep(time.Duration(7+rand.Intn(6)) * time.Second)
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
		r.log.Info("set cookie successfully",
			zap.String("cookie", c), zap.String("domain", d))
	}
}

// apiResp is the typical response of zsxq api
type apiResp struct {
	Succeeded bool `json:"succeeded"`
}

// badAPIResp zsxq api bad response
type badAPIResp struct {
	// - 1059 for too many requests
	//
	// - 401 for invalid cookie
	Code int `json:"code"`
}

// Send request with limiter, used for zsxq api.
func (r *RequestService) Limit(u string) (respByte []byte, err error) {
	logger := r.log.With(zap.String("url", u))

	logger.Info("start to get zsxq API response with limit")
	for i := 0; i < r.maxRetry; i++ {
		logger := logger.With(zap.Int("index", i))
		<-r.limiter // block until get a token

		req, err := r.setReq(u)
		if err != nil {
			logger.Error("fail to new a request", zap.Error(err))
			continue
		}

		resp, err := r.client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
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

		var respData apiResp
		if err := json.Unmarshal(bytes, &respData); err != nil {
			logger.Error("fail to unmarshal response", zap.Error(err))
			continue
		}

		if respData.Succeeded {
			return bytes, nil
		}

		var badResp badAPIResp
		if err := json.Unmarshal(bytes, &badResp); err != nil {
			logger.Error("fail to unmarshal bad resp", zap.Error(err), zap.Any("badResp", badResp))
			continue
		}
		switch badResp.Code {
		case 401:
			logger.Error("invalid cookie, clear cookie in i time")
			_ = r.redisService.Set(redis.ZsxqCookiePath, "", 0)
			return nil, ErrInvalidCookie
		case 1059:
			logger.Error("too many requests, sleep 10s")
			time.Sleep(time.Second * 10)
			continue
		default:
			logger.Error("unknown bad response", zap.Int("status_code", badResp.Code))
			continue
		}
	}

	logger.Error("fail to get zsxq API response with limit", zap.Error(err))
	return nil, err
}

// Send request with limiter, used for zsxq article
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

	logger.Error("fail to get zsxq API response with limit", zap.Error(err))
	return nil, err
}

// Send request with limiter and get a stream result,
// used for zsxq voices
func (r *RequestService) LimitStream(u string) (resp *http.Response, err error) {
	logger := r.log.With(zap.String("url", u))
	logger.Info("request with limiter for stream")

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

	logger.Error("fail to get zsxq API response with limit", zap.Error(err))
	return nil, err
}

// Send request without limiter, used for zsxq cdn
func (r *RequestService) NoLimit(u string) (respByte []byte, err error) {
	logger := r.log.With(zap.String("url", u))
	logger.Info("without limiter", zap.String("url", u))
	for i := 0; i < r.maxRetry; i++ {
		logger := logger.With(zap.Int("index", i))

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
			r.log.Error("fail to request url", zap.Error(err))
			continue
		}

		bytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err == nil {
			return bytes, nil
		}
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
	return nil, nil
}
