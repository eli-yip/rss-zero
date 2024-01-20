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
	"go.uber.org/zap"
	"golang.org/x/net/publicsuffix"
)

var (
	ErrBadResponse   = errors.New("bad response")
	ErrInvalidCookie = errors.New("invalid cookie")
	ErrMaxRetry      = errors.New("max retry")
)

const UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

type RequestService struct {
	cookies      string
	client       *http.Client
	emptyClient  *http.Client
	limiter      chan struct{}
	maxRetry     int
	redisService *redis.RedisService
	log          *zap.Logger
}

const defaultMaxRetry = 5

func NewRequestService(cookies string, redisService *redis.RedisService,
	logger *zap.Logger) *RequestService {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

	s := &RequestService{
		client:       &http.Client{Jar: jar},
		emptyClient:  &http.Client{},
		limiter:      make(chan struct{}),
		maxRetry:     defaultMaxRetry,
		redisService: redisService,
		log:          logger,
	}

	s.SetCookies(cookies)

	go func() {
		for {
			s.limiter <- struct{}{}
			time.Sleep(time.Duration(7+rand.Intn(6)) * time.Second)
		}
	}()

	return s
}

func (r *RequestService) SetCookies(cookies string) {
	r.cookies = cookies

	var domains []string = []string{
		"articles.zsxq.com",
		"api.zsxq.com",
	}

	for _, d := range domains {
		u, _ := url.Parse("https://" + d)
		for _, cookieStr := range strings.SplitN(cookies, ";", -1) {
			parts := strings.SplitN(strings.TrimSpace(cookieStr), "=", 2)
			if len(parts) == 2 {
				cookies := &http.Cookie{Name: parts[0], Value: parts[1]}
				r.client.Jar.SetCookies(u, []*http.Cookie{cookies})
			}
		}
		r.log.Info("set cookies", zap.String("cookies", cookies), zap.String("domain", d))
	}
}

// Commented out because it's not used.
// func (r *RequestService) SetMaxRetry(maxRetryTimes int) { r.maxRetry = maxRetryTimes }

// Resp is the typical response of zsxq api
type Resp struct {
	Succeeded bool `json:"succeeded"`
}

// OtherResp is the response of zsxq api when error
type OtherResp struct {
	// - 1059 for too many requests
	//
	// - 401 for invalid cookies
	Code int `json:"code"`
}

func (r *RequestService) WithLimiter(targetURL string) (respByte []byte, err error) {
	r.log.Info("with limiter", zap.String("url", targetURL))
	for i := 0; i < r.maxRetry; i++ {
		<-r.limiter
		var resp *http.Response
		req, err := http.NewRequest("GET", targetURL, nil)
		if err != nil {
			r.log.Error("new request error in i time", zap.Error(err), zap.Int("i", i))
			continue
		}
		req.Header.Set("User-Agent", UserAgent)
		req.Header.Set("Referer", "https://wx.zsxq.com/")
		resp, err = r.client.Do(req)
		// Close response body when error.
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
			r.log.Error("request error in i time", zap.Error(err), zap.Int("i", i))
			continue
		}

		bytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			r.log.Error("read response body in i time", zap.Error(err), zap.Int("i", i))
			continue
		}
		var respData Resp
		if err := json.Unmarshal(bytes, &respData); err != nil {
			r.log.Error("unmarshal response body in i time", zap.Error(err), zap.Int("i", i))
			continue
		}
		if respData.Succeeded {
			return bytes, nil
		} else {
			var otherResp OtherResp
			if err := json.Unmarshal(bytes, &otherResp); err != nil {
				r.log.Error("unmarshal other response body in i time", zap.Error(err), zap.Int("i", i))
				continue
			}
			switch otherResp.Code {
			case 1059:
				r.log.Error("too many requests, sleep 10s in i time", zap.Int("i", i))
				time.Sleep(time.Second * 10)
				continue
			case 401:
				r.log.Error("invalid cookies, clear cookies in i time", zap.Int("i", i))
				r.redisService.Set("cookies", "", 0)
				return nil, ErrInvalidCookie
			default:
				continue
			}
		}
	}

	if err == nil {
		err = ErrMaxRetry
	}
	return nil, err
}

func (r *RequestService) WithLimiterRawData(targetURL string) (respByte []byte, err error) {
	r.log.Info("with limiter raw data", zap.String("url", targetURL))
	for i := 0; i < r.maxRetry; i++ {
		<-r.limiter
		req, err := http.NewRequest("GET", targetURL, nil)
		if err != nil {
			r.log.Error("new request error in i time", zap.Error(err), zap.Int("i", i))
			continue
		}
		req.Header.Set("User-Agent", UserAgent)
		req.Header.Set("Referer", "https://wx.zsxq.com/")
		var resp *http.Response
		resp, err = r.client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
			r.log.Error("request error in i time", zap.Error(err), zap.Int("i", i))
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			r.log.Error("read response body in i time", zap.Error(err), zap.Int("i", i))
			continue
		}
		return body, nil
	}

	if err == nil {
		err = ErrMaxRetry
	}
	return nil, err
}

func (r *RequestService) WithLimiterStream(targetURL string) (resp *http.Response, err error) {
	r.log.Info("with limiter", zap.String("url", targetURL))
	for i := 0; i < r.maxRetry; i++ {
		req, err := http.NewRequest("GET", targetURL, nil)
		if err != nil {
			r.log.Error("new request error in i time", zap.Error(err), zap.Int("i", i))
			continue
		}
		req.Header.Set("User-Agent", UserAgent)
		req.Header.Set("Referer", "https://wx.zsxq.com/")
		resp, err = r.client.Do(req)
		// When request failed or status code is not 200, error.
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
			r.log.Error("request error in i time", zap.Error(err), zap.Int("i", i))
			continue
		}

		return resp, nil
	}

	if err == nil {
		err = ErrMaxRetry
	}
	return nil, err
}

func (r *RequestService) WithoutLimiter(targetURL string) (respByte []byte, err error) {
	r.log.Info("without limiter", zap.String("url", targetURL))
	for i := 0; i < r.maxRetry; i++ {
		var resp *http.Response
		req, err := http.NewRequest("GET", targetURL, nil)
		if err != nil {
			r.log.Error("new request error in i time", zap.Error(err), zap.Int("i", i))
			continue
		}
		req.Header.Set("User-Agent", UserAgent)
		req.Header.Set("Referer", "https://wx.zsxq.com/")
		resp, err = r.client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
			r.log.Error("request error in i time", zap.Error(err), zap.Int("i", i))
			continue
		}

		bytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err == nil {
			return bytes, nil
		}
	}

	if err == nil {
		err = ErrMaxRetry
	}
	return nil, err
}
