package request

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/eli-yip/zsxq-parser/internal/redis"
	"golang.org/x/net/publicsuffix"
)

var (
	ErrBadResponse = errors.New("bad response")
	ErrMaxRetry    = errors.New("max retry")
)

type RequestService struct {
	cookies      string
	client       *http.Client
	emptyClient  *http.Client
	limiter      chan struct{}
	maxRetry     int
	redisService *redis.RedisService
}

const defaultMaxRetry = 5

func NewRequestService(cookies string, redisService *redis.RedisService) *RequestService {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

	s := &RequestService{
		client:       &http.Client{Jar: jar},
		emptyClient:  &http.Client{},
		limiter:      make(chan struct{}),
		maxRetry:     defaultMaxRetry,
		redisService: redisService,
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
	}
}

func (r *RequestService) SetMaxRetry(maxRetryTimes int) { r.maxRetry = maxRetryTimes }

type Resp struct {
	Succeeded bool `json:"succeeded"`
	Raw       json.RawMessage
}

type OtherResp struct {
	Code int `json:"code"` // 1059 for too many requests, 401 for invalid cookies
}

func (r *RequestService) WithLimiterRawData(targetURL string) (respByte []byte, err error) {
	for i := 0; i < r.maxRetry; i++ {
		<-r.limiter
		req, err := http.NewRequest("GET", targetURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Referer", "https://wx.zsxq.com/")
		var resp *http.Response
		resp, err = r.client.Do(req)
		// When request failed or status code is not 200, error
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}

		return body, nil
	}
	return nil, err
}

func (r *RequestService) WithLimiter(targetURL string) (respByte []byte, err error) {
	// TODO: Rewrite this function to check status code in resp
	for i := 0; i < r.maxRetry; i++ {
		<-r.limiter
		var resp *http.Response
		req, err := http.NewRequest("GET", targetURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Referer", "https://wx.zsxq.com/")
		resp, err = r.client.Do(req)
		// When request failed or status code is not 200, error.
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
			continue
		}

		var buffer bytes.Buffer
		_, err = buffer.ReadFrom(resp.Body)
		if err != nil {
			continue
		}
		resp.Body.Close()
		bytes := buffer.Bytes()
		var respData Resp
		if err := json.Unmarshal(bytes, &respData); err != nil {
			continue
		}
		if respData.Succeeded {
			return bytes, nil
		} else {
			var otherResp OtherResp
			if err := json.Unmarshal(respData.Raw, &otherResp); err != nil {
				continue
			}
			switch otherResp.Code {
			case 1059:
				time.Sleep(time.Second * 10)
				continue
			case 401:
				r.redisService.Set("cookies", "", 0)
				return nil, ErrBadResponse
			default:
				continue
			}
		}
	}
	return nil, err
}

func (r *RequestService) WithLimiterStream(targetURL string) (resp *http.Response, err error) {
	// TODO: Rewrite this function to stay same logic with WithLimiter
	for i := 0; i < r.maxRetry; i++ {
		req, err := http.NewRequest("GET", targetURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Referer", "https://wx.zsxq.com/")
		resp, err = r.client.Do(req)
		// When request failed or status code is not 200, error.
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}

		return resp, nil
	}

	return nil, ErrBadResponse
}

func (r *RequestService) WithoutLimiter(targetURL string) (respByte []byte, err error) {
	for i := 0; i < r.maxRetry; i++ {
		var resp *http.Response
		req, err := http.NewRequest("GET", targetURL, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Referer", "https://wx.zsxq.com/")
		resp, err = r.client.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
			continue
		}

		var buffer bytes.Buffer
		_, err = buffer.ReadFrom(resp.Body)
		if err == nil {
			return buffer.Bytes(), nil
		}
	}

	return nil, err
}
