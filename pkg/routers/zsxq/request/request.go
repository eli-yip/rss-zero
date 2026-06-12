package request

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"time"

	"github.com/rs/xid"
	"go.uber.org/zap"
	"golang.org/x/net/publicsuffix"
)

var TokenPool chan struct{} = make(chan struct{})

func init() {
	go func() {
		for {
			TokenPool <- struct{}{}
			time.Sleep(time.Duration(30+rand.IntN(6)) * time.Second)
		}
	}()
}

var (
	ErrBadResponse   = errors.New("bad response")
	ErrInvalidCookie = errors.New("invalid cookie")
	ErrMaxRetry      = errors.New("max retry")
)

const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

type Requester interface {
	// Limit requests to the given url with limiter and returns data,
	// and it will validate the response json data
	Limit(context.Context, string, *zap.Logger) ([]byte, error)
	// LimitRaw requests to the given url with limiter and returns raw data,
	LimitRaw(context.Context, string, *zap.Logger) ([]byte, error)
	// LimitStream requests to the given url with limiter and returns http response,
	// Commonly used in file download
	LimitStream(context.Context, string, *zap.Logger) (*http.Response, error)
}

type RequestService struct {
	client   *http.Client
	limiter  <-chan struct{}
	maxRetry int
	logger   *zap.Logger
}

func NewRequestService(cookie string, logger *zap.Logger) Requester {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

	const defaultMaxRetry = 5
	s := &RequestService{
		client:   &http.Client{Jar: jar},
		limiter:  TokenPool,
		maxRetry: defaultMaxRetry,
		logger:   logger,
	}

	s.SetCookies(cookie)

	return s
}

func (r *RequestService) SetCookies(c string) {
	var domains = []string{
		"articles.zsxq.com",
		"api.zsxq.com",
	}

	for _, d := range domains {
		u, _ := url.Parse("https://" + d)
		cookie := &http.Cookie{Name: "zsxq_access_token", Value: c}
		r.client.Jar.SetCookies(u, []*http.Cookie{cookie})
		r.logger.Info("Set zsxq cookie successfully",
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

// {"succeeded":false,"code":1050,"info":"数据系统升级中，暂时无法操作","resp_data":{},"error":"数据系统升级中，暂时无法操作"}
type dataSystemResp struct {
	Code      int    `json:"code"`
	Succeeded bool   `json:"succeeded"`
	Info      string `json:"info"`
	Error     string `json:"error"`
}

// zsxq API business codes carried in a 200 response body.
const (
	codeInvalidCookie   = 401  // invalid cookie
	codeTooManyRequests = 1059 // too many requests due to no sign
	codeDataSystemBusy  = 1050 // data system upgrading
)

// noSignBackoff is how long to wait after a too-many-requests (1059) response.
const noSignBackoff = 60 * time.Second

// doWithRetry runs a limited+retried GET that must yield HTTP 200, and is the
// single source of truth for the retry loop shared by Limit/LimitRaw/LimitStream.
//
// On each 200 it calls validate(resp):
//   - done=true  -> stop, return this resp (the caller owns resp.Body)
//   - done=false -> retry (validate must Close resp.Body if it read it)
//   - err!=nil   -> fatal, return immediately (e.g. ErrInvalidCookie)
//
// Transport errors and non-200 responses are retried; their bodies are closed
// before retrying. After maxRetry attempts it returns ErrMaxRetry, wrapping the
// last underlying error for diagnostics.
func (r *RequestService) doWithRetry(ctx context.Context, u string,
	validate func(resp *http.Response) (done bool, err error),
	logger *zap.Logger) (*http.Response, error) {
	requestTaskID := xid.New().String()
	logger.Info("Start to request zsxq api, waiting for limiter", zap.String("url", u), zap.String("request_task_id", requestTaskID))

	var err error
	for i := 0; i < r.maxRetry; i++ {
		currentRequestTaskID := fmt.Sprintf("%s_%d", requestTaskID, i)
		logger := logger.With(zap.String("request_task_id", currentRequestTaskID))
		<-r.limiter // block until get a token
		logger.Info("Get limiter successfully, start to request url")

		var req *http.Request
		if req, err = r.setReq(ctx, u); err != nil {
			logger.Error("Failed to new a request", zap.Error(err))
			continue
		}

		var resp *http.Response
		if resp, err = r.client.Do(req); err != nil {
			logger.Error("Failed to request url", zap.Error(err))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			err = fmt.Errorf("bad response status code: %d", resp.StatusCode)
			logger.Error("Bad status code", zap.Error(err))
			continue
		}

		// 200 reached: transport succeeded. Clear err so a subsequent business
		// retry (validate -> done=false) ends as ErrMaxRetry, not a stale error.
		err = nil

		var done bool
		if done, err = validate(resp); err != nil {
			return nil, err
		}
		if done {
			return resp, nil
		}
	}

	if err == nil {
		return nil, ErrMaxRetry
	}
	err = fmt.Errorf("%w: %w", ErrMaxRetry, err)
	logger.Error("Failed to request zsxq API", zap.Error(err))
	return nil, err
}

// Limit sends a request with limiter and validates the zsxq business JSON.
func (r *RequestService) Limit(ctx context.Context, u string, logger *zap.Logger) ([]byte, error) {
	var body []byte
	validate := func(resp *http.Response) (bool, error) {
		bytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			logger.Error("Failed to read response body", zap.Error(err))
			return false, nil
		}

		var respData apiResp
		if err = json.Unmarshal(bytes, &respData); err != nil {
			logger.Error("Failed to unmarshal response into apiResp", zap.Error(err))
			return false, nil
		}
		if respData.Succeeded {
			body = bytes
			return true, nil
		}

		var badResp badAPIResp
		if err = json.Unmarshal(bytes, &badResp); err != nil {
			logger.Error("Failed to unmarshal bad resp", zap.Error(err))
			return false, nil
		}

		switch badResp.Code {
		case codeInvalidCookie:
			logger.Error("Invalid zsxq cookie")
			return false, ErrInvalidCookie
		case codeTooManyRequests:
			logger.Warn("Too many requests due to no sign, sleep 60s")
			time.Sleep(noSignBackoff)
			return false, nil
		case codeDataSystemBusy:
			var dataSystemResp dataSystemResp
			if err = json.Unmarshal(bytes, &dataSystemResp); err != nil {
				logger.Error("Failed to unmarshal data system resp", zap.Error(err))
				return false, nil
			}
			logger.Info("Data system of zsxq is upgrading or other reasons", zap.String("info", dataSystemResp.Info), zap.String("error", dataSystemResp.Error))
			return false, nil
		default:
			logger.Error("Unknown bad response", zap.Int("status_code", badResp.Code), zap.String("response", string(bytes)))
			return false, nil
		}
	}

	if _, err := r.doWithRetry(ctx, u, validate, logger); err != nil {
		return nil, err
	}
	return body, nil
}

// LimitRaw sends a request with limiter and returns the raw body, used for zsxq article.
func (r *RequestService) LimitRaw(ctx context.Context, u string, logger *zap.Logger) ([]byte, error) {
	var body []byte
	validate := func(resp *http.Response) (bool, error) {
		bytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			logger.Error("Failed to read response body", zap.Error(err))
			return false, nil
		}
		body = bytes
		return true, nil
	}

	if _, err := r.doWithRetry(ctx, u, validate, logger); err != nil {
		return nil, err
	}
	return body, nil
}

// LimitStream sends a request with limiter and hands off the http response,
// commonly used for file/voice download. The caller owns resp.Body.
func (r *RequestService) LimitStream(ctx context.Context, u string, logger *zap.Logger) (*http.Response, error) {
	return r.doWithRetry(ctx, u, func(*http.Response) (bool, error) { return true, nil }, logger)
}

// Set request header and method
func (r *RequestService) setReq(ctx context.Context, u string) (req *http.Request, err error) {
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://wx.zsxq.com/")
	return req, nil
}
