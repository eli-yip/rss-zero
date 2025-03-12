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
	// NoLimit requests to the given url without limiter
	NoLimit(context.Context, string, *zap.Logger) ([]byte, error)
}

type RequestService struct {
	client      *http.Client
	emptyClient *http.Client
	limiter     <-chan struct{}
	maxRetry    int
	logger      *zap.Logger
}

func NewRequestService(cookie string, logger *zap.Logger) Requester {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

	const defaultMaxRetry = 5
	s := &RequestService{
		client:      &http.Client{Jar: jar},
		emptyClient: &http.Client{},
		limiter:     TokenPool,
		maxRetry:    defaultMaxRetry,
		logger:      logger,
	}

	s.SetCookies(cookie)

	return s
}

func (r *RequestService) SetCookies(c string) {
	var domains []string = []string{
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

// Send request with limiter, used for zsxq api.
func (r *RequestService) Limit(ctx context.Context, u string, logger *zap.Logger) (respByte []byte, err error) {
	requestTaskID := xid.New().String()
	logger.Info("Start to reqeust zsxq api, waiting for limiter", zap.String("url", u), zap.String("request_task_id", requestTaskID))

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
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("bad response status code: %d", resp.StatusCode)
			logger.Error("Bad status code", zap.Error(err))
			continue
		}

		var bytes []byte
		if bytes, err = io.ReadAll(resp.Body); err != nil {
			logger.Error("Failed to read response body", zap.Error(err))
			continue
		}

		var respData apiResp
		if err = json.Unmarshal(bytes, &respData); err != nil {
			logger.Error("Failed to unmarshal response into apiResp", zap.Error(err))
			continue
		}

		if respData.Succeeded {
			return bytes, nil
		}

		var badResp badAPIResp
		if err = json.Unmarshal(bytes, &badResp); err != nil {
			logger.Error("Failed to unmarshal bad resp", zap.Error(err), zap.Any("badResp", badResp))
			continue
		}

		switch badResp.Code {
		case 401:
			logger.Error("Invalid zsxq cookie")
			return nil, ErrInvalidCookie
		case 1059:
			logger.Warn("Too many requests due to no sign, sleep 60s")
			time.Sleep(60 * time.Second)
			continue
		case 1050:
			var dataSystemResp dataSystemResp
			if err = json.Unmarshal(bytes, &dataSystemResp); err != nil {
				logger.Error("Failed to unmarshal data system resp", zap.Error(err))
				continue
			}
			logger.Info("Data system of zsxq is upgrading or other reasons", zap.String("info", dataSystemResp.Info), zap.String("error", dataSystemResp.Error))
			continue
		default:
			logger.Error("Unknown bad response", zap.Int("status_code", badResp.Code), zap.String("response", string(bytes)))
			continue
		}
	}

	if err == nil {
		err = ErrMaxRetry
	}
	logger.Error("Failed to request zsxq API with limit", zap.Error(err))
	return nil, err
}

// Send request with limiter, used for zsxq article
func (r *RequestService) LimitRaw(ctx context.Context, u string, logger *zap.Logger) (respByte []byte, err error) {
	requestTaskID := xid.New().String()
	logger.Info("Start to reqeust zsxq api raw, waiting for limiter", zap.String("url", u), zap.String("request_task_id", requestTaskID))

	for i := 0; i < r.maxRetry; i++ {
		currentRequestTaskID := fmt.Sprintf("%s_%d", requestTaskID, i)
		logger := logger.With(zap.String("request_task_id", currentRequestTaskID))
		<-r.limiter
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
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("bad status code: %d", resp.StatusCode)
			logger.Error("Bad status code", zap.Error(err))
			continue
		}

		var body []byte
		if body, err = io.ReadAll(resp.Body); err != nil {
			logger.Error("Failed to read response body", zap.Error(err))
			continue
		}
		return body, nil
	}

	if err == nil {
		err = ErrMaxRetry
	}
	logger.Error("Failed to get zsxq API response with limit", zap.Error(err))
	return nil, err
}

// Send request with limiter and get a stream result,
// used for zsxq voices
func (r *RequestService) LimitStream(ctx context.Context, u string, logger *zap.Logger) (resp *http.Response, err error) {
	requestTaskID := xid.New().String()
	logger.Info("Start to reqeust zsxq api stream, waiting for limiter", zap.String("url", u), zap.String("request_task_id", requestTaskID))

	for i := 0; i < r.maxRetry; i++ {
		currentRequestTaskID := fmt.Sprintf("%s_%d", requestTaskID, i)
		logger := logger.With(zap.String("request_task_id", currentRequestTaskID))
		<-r.limiter
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
			defer resp.Body.Close()
			err = fmt.Errorf("bad status code: %d", resp.StatusCode)
			logger.Error("bad status code", zap.Error(err))
			continue
		}

		return resp, nil
	}

	if err == nil {
		err = ErrMaxRetry
	}
	logger.Error("Failed to get zsxq API response with limit", zap.Error(err))
	return nil, err
}

// Send request without limiter, used for zsxq cdn
func (r *RequestService) NoLimit(ctx context.Context, u string, logger *zap.Logger) (respByte []byte, err error) {
	requestTaskID := xid.New().String()
	logger.Info("Start to request zsxq api without limit", zap.String("url", u), zap.String("request_task_id", requestTaskID))

	for i := 0; i < r.maxRetry; i++ {
		currentRequestTaskID := fmt.Sprintf("%s_%d", requestTaskID, i)
		logger := logger.With(zap.String("request_task_id", currentRequestTaskID))

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
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("bad status code: %d", resp.StatusCode)
			logger.Error("Bad status code", zap.Error(err))
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
	logger.Error("Failed to get zsxq API response without limit", zap.Error(err))
	return nil, err
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
