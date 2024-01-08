package request

import (
	"bytes"
	"encoding/json"
	"errors"
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

func NewRequestService(cookies string, redisService *redis.RedisService) *RequestService {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

	rs := &RequestService{
		client:       &http.Client{Jar: jar},
		emptyClient:  &http.Client{},
		limiter:      make(chan struct{}),
		maxRetry:     3,
		redisService: redisService,
	}

	rs.SetCookies(cookies)

	go func() {
		for {
			time.Sleep(time.Duration(5+rand.Intn(6)) * time.Second)
			rs.limiter <- struct{}{}
		}
	}()

	return rs
}

func (r *RequestService) SetCookies(cookies string) {
	r.cookies = cookies
	u, _ := url.Parse("https://api.zsxq.com")
	for _, cookieStr := range strings.SplitN(cookies, ";", -1) {
		parts := strings.SplitN(strings.TrimSpace(cookieStr), "=", 2)
		if len(parts) == 2 {
			cookies := &http.Cookie{Name: parts[0], Value: parts[1]}
			r.client.Jar.SetCookies(u, []*http.Cookie{cookies})
		}
	}
}

func (r *RequestService) SetMaxRetry(maxRetryTimes int) { r.maxRetry = maxRetryTimes }

type Resp struct {
	Succeeded bool `json:"succeeded"`
}

func (r *RequestService) WithLimiter(targetURL string) (respByte []byte, err error) {
	for i := 0; i < r.maxRetry; i++ {
		<-r.limiter
		var resp *http.Response
		resp, err = r.client.Get(targetURL)
		// When request failed or status code is not 200, error.
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
			// If error is nil when status code is not 200, set it to ErrBadResponse.
			if err == nil {
				err = ErrBadResponse
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
		}
	}
	return nil, err
}

func (r *RequestService) WithoutLimiter(targetURL string) (respByte []byte, err error) {
	for i := 0; i < r.maxRetry; i++ {
		var resp *http.Response
		resp, err = r.emptyClient.Get(targetURL)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
			if err == nil {
				err = ErrBadResponse
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

func (r *RequestService) WithLimiterStream(targetURL string) (resp *http.Response, err error) {
	for i := 0; i < r.maxRetry; i++ {
		resp, err = r.emptyClient.Get(targetURL)
		if err != nil {
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
