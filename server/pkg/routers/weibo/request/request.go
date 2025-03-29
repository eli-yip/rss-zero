package request

import (
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
)

type Requester interface {
	LimitRaw(u string) ([]byte, error)
	GetPicStream(u string) (*http.Response, error)
}

type RequestService struct {
	client       *http.Client
	limiter      chan struct{}
	maxRetry     int
	redisService *redis.Redis
	logger       *zap.Logger
}

func NewRequestService(redisService redis.Redis, cookie string, logger *zap.Logger) (Requester, error) {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	const defaultMaxRetry = 5

	rs := &RequestService{
		client:       &http.Client{Jar: jar},
		redisService: &redisService,
		limiter:      make(chan struct{}),
		maxRetry:     defaultMaxRetry,
		logger:       logger,
	}

	rs.setCookies(cookie)

	go func() {
		for {
			rs.limiter <- struct{}{}
			time.Sleep(time.Duration(30+rand.IntN(6)) * time.Second)
		}
	}()

	return rs, nil
}

func (rs *RequestService) setCookies(cookie string) {
	domains := []string{"weibo.com"}

	for _, domain := range domains {
		u, _ := url.Parse("https://" + domain)
		for _, cp := range strings.SplitN(cookie, ";", -1) {
			if n, v, ok := strings.Cut(strings.TrimSpace(cp), "="); ok {
				cookie := &http.Cookie{Name: n, Value: v}
				rs.client.Jar.SetCookies(u, []*http.Cookie{cookie})
			}
		}
	}
}

func (rs *RequestService) LimitRaw(u string) (data []byte, err error) {
	logger := rs.logger.With(zap.String("url", u))

	logger.Info("start to get weibo raw data")

	for i := 0; i < rs.maxRetry; i++ {
		logger := logger.With(zap.Int("index", i))
		<-rs.limiter

		var req *http.Request
		if req, err = rs.setReq(u); err != nil {
			logger.Error("failed to new a request", zap.Error(err))
			continue
		}

		var resp *http.Response
		if resp, err = rs.client.Do(req); err != nil {
			logger.Error("failed to request url", zap.Error(err))
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("bad status code: %d", resp.StatusCode)
			logger.Error("bad status code", zap.Int("code", resp.StatusCode))
			continue
		}

		var body []byte
		if body, err = io.ReadAll(resp.Body); err != nil {
			logger.Error("failed to read response body", zap.Error(err))
			continue
		}
		return body, nil
	}

	if err == nil {
		return nil, fmt.Errorf("reach max retry limit")
	}
	logger.Error("failed to get weibo api response", zap.Error(err))
	return nil, err
}

func (rs *RequestService) setReq(u string) (req *http.Request, err error) {
	if req, err = http.NewRequest(http.MethodGet, u, nil); err != nil {
		return nil, fmt.Errorf("failed to new a request: %w", err)
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3")
	return req, nil
}

func (rs *RequestService) GetPicStream(u string) (resp *http.Response, err error) {
	if resp, err = rs.getPicStream(u); err != nil {
		return nil, fmt.Errorf("failed to get picture stream: %w", err)
	}
	return resp, nil
}

func (rs *RequestService) getPicStream(picLink string) (resp *http.Response, err error) {
	req, err := http.NewRequest("GET", picLink, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to new a request: %w", err)
	}
	req.Header.Set("Referer", "https://weibo.com")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request url: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}

	return resp, nil
}
