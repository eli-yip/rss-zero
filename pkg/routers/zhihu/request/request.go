package request

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"slices"
	"time"

	"github.com/rs/xid"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/notify"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
)

var TokenCh chan struct{} = make(chan struct{})

func init() {
	go func() {
		for {
			TokenCh <- struct{}{}
			time.Sleep(time.Duration(300+rand.IntN(300)) * time.Second)
		}
	}()
}

type Requester interface {
	// LimitRaw requests to the given url with limiter and returns raw data,
	LimitRaw(context.Context, string, *zap.Logger) ([]byte, error)
	// NoLimitRaw requests to the given url without limiter and returns raw data,
	// Commonly used in file download
	NoLimitStream(context.Context, string, *zap.Logger) (*http.Response, error)
}

var (
	ErrBadResponse      = errors.New("bad response")
	ErrMaxRetry         = errors.New("max retry")
	ErrUnreachable      = errors.New("unreachable")
	ErrNeedZC0          = errors.New("need login")
	ErrInvalidZSECK     = errors.New("invalid zse_ck")
	ErrInvalidZC0       = errors.New("invalid z_c0")
	ErrForbidden        = errors.New("forbidden")
	ErrAccountDestroyed = errors.New("account destroyed")
)

const (
	userAgent    = "ZhihuHybrid com.zhihu.android/Futureve/6.59.0 Mozilla/5.0 (Linux; Android 10; GM1900 Build/QKQ1.190716.003; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/85.0.4183.127 Mobile Safari/537.36"
	apiUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Safari/537.36 Edg/87.0.664.75"
)

type RequestService struct {
	client             *http.Client
	limiter            <-chan struct{}
	maxRetry           int // default 5
	logger             *zap.Logger
	d_c0, z_c0, zse_ck string
	dbService          zhihuDB.EncryptionServiceIface
	notify             notify.Notifier
}

func NewLimiter() <-chan struct{} {
	var tokenCh = make(chan struct{})
	go func() {
		for {
			tokenCh <- struct{}{}
			time.Sleep(time.Duration(300+rand.IntN(300)) * time.Second)
		}
	}()
	return tokenCh
}

type OptionFunc func(*RequestService)

func WithLimiter(limiter <-chan struct{}) OptionFunc {
	return func(r *RequestService) {
		r.limiter = limiter
	}
}

type Cookie struct{ DC0, ZseCk, ZC0 string }

func NewRequestService(logger *zap.Logger, dbService zhihuDB.EncryptionServiceIface, notifier notify.Notifier, cookie *Cookie, opts ...OptionFunc) (Requester, error) {
	const defaultMaxRetry = 5

	s := &RequestService{
		client:    &http.Client{},
		limiter:   TokenCh,
		maxRetry:  defaultMaxRetry,
		dbService: dbService,
		notify:    notifier,
		zse_ck:    cookie.ZseCk,
		d_c0:      cookie.DC0,
		z_c0:      cookie.ZC0,
		logger:    logger,
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

type Error403 struct {
	Error struct {
		NeedLogin bool   `json:"need_login"`
		Message   string `json:"message"`
		Code      int    `json:"code"`
	} `json:"error"`
}

type Error401 struct {
	Error struct {
		Code    int    `json:"code"`
		Name    string `json:"name"`
		Message string `json:"message"`
	} `json:"error"`
}

type ErrorResp struct {
	Error struct {
		Code    int    `json:"code"`
		Name    string `json:"name"`
		Message string `json:"message"`
	} `json:"error"`
}

// z_c0 error messages in 401 response
var zC0ErrMsgs = []string{
	`ERR_TICKET_NOT_EXIST`,
	`ERR_PARSE_LOGIN_TICKET`,
}

type EncryptReq struct {
	RequestID string `json:"request_id"`
	DC0       string `json:"d_c0,omitempty"`
	ZC0       string `json:"z_c0,omitempty"`
	ZSE_CK    string `json:"zse_ck,omitempty"`
	URL       string `json:"url"`
}

type EncryptErrResp struct {
	Error string `json:"error"`
}

// UpstreamNotJSONResp is the body the encryption service returns with a 502
// when zhihu responded with a non-JSON body. It forwards zhihu's real status
// code and raw body so we can decide whether to retry or refresh cookies.
type UpstreamNotJSONResp struct {
	ZhihuStatus int    `json:"zhihu_status"`
	RawBody     string `json:"raw_body"`
	Error       string `json:"error"`
}

// Send request with limiter, used for all zhihu api requests
func (r *RequestService) LimitRaw(ctx context.Context, u string, logger *zap.Logger) (respByte []byte, err error) {
	requestTaskID := xid.New().String()
	logger.Info("Start to get zhihu raw data with limit, waiting for limiter", zap.String("url", u), zap.String("request_task_id", requestTaskID))

	for i := range r.maxRetry {
		currentRequestTaskID := fmt.Sprintf("%s_%d", requestTaskID, i)
		logger := logger.With(zap.String("request_task_id", currentRequestTaskID))

		es, err := r.dbService.SelectService()
		if err != nil {
			logger.Error("Failed to Select encryption service", zap.Error(err))
			if errors.Is(err, zhihuDB.ErrNoAvailableService) {
				return nil, err
			}
			continue
		}
		logger.Info("Select zhihu encryption service successfully", zap.Any("service", es))

		<-r.limiter
		logger.Info("Get limiter successfully, start to request url")

		reqBodyByte, err := json.Marshal(EncryptReq{RequestID: currentRequestTaskID, DC0: r.d_c0, ZC0: r.z_c0, ZSE_CK: r.zse_ck, URL: u})
		if err != nil {
			logger.Error("Failed to marshal request body", zap.Error(err))
			continue
		}

		if err = r.dbService.IncreaseUsedCount(es.ID); err != nil {
			logger.Error("Failed to increase used count", zap.Error(err))
			return nil, fmt.Errorf("failed to increase used count for service %s, %w", es.ID, err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, es.URL+"/data", bytes.NewBuffer(reqBodyByte))
		if err != nil {
			logger.Error("Failed to new a request", zap.Error(err))
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := r.client.Do(req)
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

		if isAccountDestroyedResp(body) {
			logger.Error("Zhihu account has been destroyed", zap.String("resp_body", string(body)))
			return nil, ErrAccountDestroyed
		}

		switch resp.StatusCode {
		case http.StatusOK:
			logger.Info("Get zhihu raw data successfully")
			return body, nil
		case http.StatusForbidden:
			logger.Error("Get 403 error", zap.String("resp_body", string(body)))
			if err = r.dbService.IncreaseFailedCount(es.ID); err != nil {
				logger.Error("Failed to increase failed count", zap.Error(err))
			}
			logger.Info("Increase encryption service failed count successfully")
			var e403 Error403
			if err = json.Unmarshal(body, &e403); err != nil {
				logger.Error("Failed to unmarshal 403 error", zap.Error(err))
				continue
			}
			switch {
			case e403.Error.NeedLogin:
				logger.Error("Need login")
				if err = r.dbService.MarkUnavailable(es.ID); err != nil {
					logger.Error("Failed to mark unavailable", zap.Error(err))
				}
				logger.Info("Mark encryption service unavailable successfully")
				return nil, ErrNeedZC0
			case e403.Error.Code == 40362:
				// {"error":{"message":"您当前请求存在异常，暂时限制本次访问。如有疑问，您可以通过手机摇一摇或登录后私信知乎小管家反馈。","code":40362}}
				message := func() string {
					if e403.Error.Message != "" {
						return e403.Error.Message
					}
					return "您当前请求存在异常，暂时限制本次访问。如有疑问，您可以通过手机摇一摇或登录后私信知乎小管家反馈。"
				}()
				logger.Error("Need to refresh __zse_ck cookie", zap.String("message", message))
				return nil, ErrInvalidZSECK
			default:
				return nil, ErrForbidden
			}
		case http.StatusUnauthorized:
			logger.Error("Get 401 error", zap.String("resp_body", string(body)))
			var errResp Error401
			if err = json.NewDecoder(bytes.NewBuffer(body)).Decode(&errResp); err != nil {
				logger.Error("Failed to unmarshal 401 error", zap.Error(err))
				continue
			}
			if errResp.Error.Code == 100 && slices.Contains(zC0ErrMsgs, errResp.Error.Message) {
				logger.Error("Invalid z_c0 cookie")
				return nil, ErrInvalidZC0
			}
			logger.Error("Invalid __zse_ck cookie")
			return nil, ErrInvalidZSECK
		case http.StatusNotFound:
			logger.Error("Get 404 error")
			if err = r.dbService.IncreaseFailedCount(es.ID); err != nil {
				logger.Error("Failed to increase failed count", zap.Error(err))
			}
			return nil, ErrUnreachable
		case http.StatusNotImplemented:
			logger.Error("Get 501 error", zap.String("resp_body", string(body)))
			if err = r.dbService.IncreaseFailedCount(es.ID); err != nil {
				logger.Error("Failed to increase failed count", zap.Error(err))
			}
			var encryptErrResp EncryptErrResp
			if err = json.Unmarshal(body, &encryptErrResp); err != nil {
				logger.Error("Failed to unmarshal 501 error", zap.Error(err))
			}
			logger.Error("501 error", zap.String("error", encryptErrResp.Error))
			return nil, ErrBadResponse
		case http.StatusBadGateway:
			// 502 means the encryption service could not parse zhihu's response
			// as JSON. It forwards zhihu's real status code and raw body. For now
			// just log it and retry with a fresh service, rather than guessing the
			// cause (e.g. assuming __zse_ck expired) — we need real-world samples
			// of zhihu_status/raw_body before deciding how to branch on them.
			var upstream UpstreamNotJSONResp
			if uerr := json.Unmarshal(body, &upstream); uerr != nil {
				logger.Error("Failed to unmarshal 502 error", zap.Error(uerr), zap.String("resp_body", string(body)))
			}
			logger.Error("Zhihu returned non-json body, will retry",
				zap.Int("zhihu_status", upstream.ZhihuStatus),
				zap.String("raw_body", upstream.RawBody))
			if ferr := r.dbService.IncreaseFailedCount(es.ID); ferr != nil {
				logger.Error("Failed to increase failed count", zap.Error(ferr))
			}
			continue
		default:
			logger.Error("Bad status code", zap.Int("status_code", resp.StatusCode), zap.String("resp_body", string(body)))
			continue
		}
	}

	// NOTE: err here is the function-level named return, which the loop body
	// shadows via `:=` (line `es, err := r.dbService.SelectService()`), so it is
	// always nil. Return ErrMaxRetry so callers get a meaningful, non-nil error
	// when all retries are exhausted instead of (nil, nil).
	r.logger.Error("Failed to get zhihu raw data after exhausting retries", zap.String("request_task_id", requestTaskID))
	return nil, ErrMaxRetry
}

func isAccountDestroyedResp(body []byte) bool {
	var errResp ErrorResp
	if err := json.Unmarshal(body, &errResp); err != nil {
		return false
	}

	return errResp.Error.Code == 310000 ||
		errResp.Error.Name == "AccountDestroyError" ||
		errResp.Error.Message == "该账号已注销"
}

func (r *RequestService) NoLimitStream(ctx context.Context, u string, logger *zap.Logger) (resp *http.Response, err error) {
	return NoLimitStream(ctx, r.client, u, r.maxRetry, logger)
}

func NoLimitStream(ctx context.Context, client *http.Client, u string, maxRetry int, logger *zap.Logger) (resp *http.Response, err error) {
	logger = logger.With(zap.String("url", u))
	logger.Info("start to request without limit for stream")

	for i := range maxRetry {
		logger := logger.With(zap.Int("index", i))

		var req *http.Request
		req, err = setReq(ctx, u)
		if err != nil {
			logger.Error("failed to new a request", zap.Error(err))
			continue
		}

		resp, err = client.Do(req)
		if err != nil {
			logger.Error("failed to request url", zap.Error(err))
			continue
		}
		// do not defer resp body close here because we will save it to minio

		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			err = fmt.Errorf("bad response status code: %d", resp.StatusCode)
			logger.Error("bad status code", zap.Int("status code", resp.StatusCode))
			continue
		}

		return resp, nil
	}

	logger.Error("failed to get zhihu no limit stream", zap.Error(err))
	return nil, err
}

func setReq(ctx context.Context, u string) (req *http.Request, err error) {
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	return req, nil
}
