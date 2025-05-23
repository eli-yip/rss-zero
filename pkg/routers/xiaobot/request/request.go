package request

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/encrypt"
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

type Requester interface {
	// Limit requests to the given url with limiter and returns data,
	// and it will validate the response json data
	Limit(string) ([]byte, error)
}

const userAgent = `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36`

var (
	ErrBadResponse   = errors.New("bad response")
	ErrInvalidSign   = errors.New("invalid sign")
	ErrMaxRetry      = errors.New("max retry")
	ErrNeedLogin     = errors.New("need login")
	ErrUnimplemented = errors.New("unimplemented")
)

type RequestService struct {
	client        *http.Client
	limiter       <-chan struct{}
	maxRetry      int
	token         string // token is used to request xiaobot api in authorization header
	cookieService cookie.CookieIface
	logger        *zap.Logger
}

func NewRequestService(cookieService cookie.CookieIface, token string, logger *zap.Logger) Requester {
	const defaultMaxRetry = 5
	s := &RequestService{
		client:        &http.Client{},
		limiter:       TokenPool,
		maxRetry:      defaultMaxRetry,
		cookieService: cookieService,
		token:         token,
		logger:        logger,
	}

	return s
}

func (r *RequestService) Limit(u string) (data []byte, err error) {
	logger := r.logger.With(zap.String("url", u))
	logger.Info("Start to request url with limiter")

	for i := 0; i < r.maxRetry; i++ {
		logger := logger.With(zap.Int("index", i))
		<-r.limiter

		var req *http.Request
		if req, err = r.setAPIReq(u); err != nil {
			logger.Error("Failed to create a request", zap.Error(err))
			continue
		}

		var resp *http.Response
		if resp, err = r.client.Do(req); err != nil {
			logger.Error("Failed to request url", zap.Error(err))
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			logger.Error("Status code not 200", zap.Int("status", resp.StatusCode))
			err = fmt.Errorf("status code not 200, %d", resp.StatusCode)
			continue
		}

		var bytes []byte
		if bytes, err = io.ReadAll(resp.Body); err != nil {
			logger.Error("Failed to read response body", zap.Error(err))
			continue
		}

		var apiResp baseResp
		if err = json.Unmarshal(bytes, &apiResp); err != nil {
			logger.Error("Failed to unmarshal response bytes", zap.Error(err))
			continue
		}

		if err = r.validateAPIResp(apiResp.Code, bytes, logger); err != nil {
			return nil, err
		}

		var okResp okResp
		if err = json.Unmarshal(bytes, &okResp); err != nil {
			logger.Error("Failed to unmarshal response bytes", zap.Error(err))
			return nil, err
		}
		return okResp.Data, nil
	}

	logger.Error("Failed to get xiaobot response", zap.Error(err))
	return nil, err
}

// validateAPIResp checks:
//
//  1. need sign in
//  2. bad request
//  3. sign error
//  4. ok
func (r *RequestService) validateAPIResp(code int, bytes []byte, logger *zap.Logger) (err error) {
	switch code {
	case codeNeedSignIn:
		badMessage, err := r.getErrorMessage(bytes)
		if err != nil {
			logger.Error("Failed to get error message", zap.Error(err))
			return err
		}
		logger.Error("Need sign in", zap.String("message", badMessage))
		if err := r.cookieService.Del(cookie.CookieTypeXiaobotAccessToken); err != nil {
			logger.Error("Failed to delete xiaobot token", zap.Error(err))
		}
		logger.Info("Deleted xiaobot token successfully")
		return ErrNeedLogin
	case codeBadRequest:
		badMessage, err := r.getErrorMessage(bytes)
		if err != nil {
			logger.Error("Failed to get error message", zap.Error(err))
			return err
		}
		logger.Error("Bad request", zap.String("message", badMessage))
		return ErrBadResponse
	case codeSignError:
		badMessage, err := r.getErrorMessage(bytes)
		if err != nil {
			logger.Error("Failed to get error message", zap.Error(err))
			return err
		}
		logger.Error("Sign error", zap.String("message", badMessage))
		return ErrInvalidSign
	case codeOK:
		return nil
	default:
		logger.Error("Unknown response code", zap.Int("code", code))
		return ErrBadResponse
	}
}

func (r *RequestService) getErrorMessage(bytes []byte) (message string, err error) {
	var badResp badResp
	if err = json.Unmarshal(bytes, &badResp); err != nil {
		return "", err
	}
	return badResp.Message, nil
}

// setAPIReq sets the request header for xiaobot api
//
//	 ```
//		Host:api.xiaobot.net
//		Connection:keep-alive
//		sec-ch-ua:"Chromium";v="121", "Not A(Brand";v="99"
//		app-version:0.1
//		DNT:1
//		sec-ch-ua-mobile:?0
//		Authorization:Bearer 1758768|dAJhxxxxxx8FZVup5tBztopHzMvIsW21zwD6
//		User-Agent:Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36
//		Accept:application/json, text/plain, */*
//		timestamp:1707558382
//		api-key:xiaobot_web
//		sign:2bb91f1117e3bc7a13e9a688ed7a87b0
//		sec-ch-ua-platform:"macOS"
//		Origin:https://xiaobot.net
//		Sec-Fetch-Site:same-site
//		Sec-Fetch-Mode:cors
//		Sec-Fetch-Dest:empty
//		Referer:https://xiaobot.net/
//		Accept-Encoding:gzip, deflate, br (remove this because we do not want to handle gzip)
//		Accept-Language:zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7
//	 ```
func (r *RequestService) setAPIReq(u string) (req *http.Request, err error) {
	req, err = http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}

	timestamp, sign, err := encrypt.Sign(time.Now(), u)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Host", "api.xiaobot.net")
	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("sec-ch-ua", `"Chromium";v="121", "Not A(Brand";v="99"`)
	req.Header.Add("app-version", "0.1")
	req.Header.Add("DNT", "1")
	req.Header.Add("sec-ch-ua-mobile", "?0")
	req.Header.Add("Authorization", "Bearer "+r.token)
	req.Header.Add("User-Agent", userAgent)
	req.Header.Add("Accept", "application/json, text/plain, */*")
	req.Header.Add("timestamp", timestamp)
	req.Header.Add("api-key", "xiaobot_web")
	req.Header.Add("sign", sign)
	req.Header.Add("sec-ch-ua-platform", `"macOS"`)
	req.Header.Add("Origin", "https://xiaobot.net")
	req.Header.Add("Sec-Fetch-Site", "same-site")
	req.Header.Add("Sec-Fetch-Mode", "cors")
	req.Header.Add("Sec-Fetch-Dest", "empty")
	req.Header.Add("Referer", "https://xiaobot.net/")
	req.Header.Add("Accept-Language", "zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7")

	return req, nil
}
