package request

import (
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"time"

	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/request"
	"github.com/eli-yip/rss-zero/pkg/routers/xiaobot/encrypt"
	"go.uber.org/zap"
)

const userAgent = `Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36`

var (
	ErrBadResponse = errors.New("bad response")
	ErrInvalidSign = errors.New("invalid sign")
	ErrMaxRetry    = errors.New("max retry")
	ErrNeedLogin   = errors.New("need login")
)

type RequestService struct {
	client       *http.Client
	limiter      chan struct{}
	maxRetry     int
	token        string
	redisService *redis.RedisService
	log          *zap.Logger
}

func NewRequestService(r *redis.RedisService, token string, logger *zap.Logger) request.Requester {
	const defaultMaxRetry = 5
	s := &RequestService{
		client:       &http.Client{},
		limiter:      make(chan struct{}),
		maxRetry:     defaultMaxRetry,
		redisService: r,
		token:        token,
		log:          logger,
	}

	go func() {
		for {
			s.limiter <- struct{}{}
			time.Sleep(time.Duration(30+rand.Intn(6)) * time.Second)
		}
	}()

	return s
}

func (r *RequestService) Limit(u string) (data []byte, err error) {
	logger := r.log.With(zap.String("url", u))
	logger.Info("Start requesting url with limiter")

	for i := 0; i < r.maxRetry; i++ {
		logger := logger.With(zap.Int("index", i))
		<-r.limiter

		req, err := r.setAPIReq(u)
		if err != nil {
			logger.Error("Failed newing a request", zap.Error(err))
			continue
		}

		resp, err := r.client.Do(req)
		if err != nil {
			logger.Error("Failed request url", zap.Error(err))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			logger.Error("Status code not 200", zap.Int("status", resp.StatusCode))
			continue
		}

		bytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			logger.Error("Failed reading response body", zap.Error(err))
			continue
		}

		var apiResp baseResp
		if err := json.Unmarshal(bytes, &apiResp); err != nil {
			logger.Error("Failed unmarshaling response bytes", zap.Error(err))
			continue
		}

		switch apiResp.Code {
		case codeNeedSignIn:
			var badResp badResp
			if err := json.Unmarshal(bytes, &badResp); err != nil {
				logger.Error("Failed unmarshaling response bytes", zap.Error(err))
				continue
			}
			logger.Error("Need sign in", zap.String("message", badResp.Message))
			if err := r.redisService.Del(redis.XiaobotTokenPath); err != nil {
				logger.Error("Failed deleting xiaobot token", zap.Error(err))
			}
			logger.Info("Deleted xiaobot token")
			return nil, ErrNeedLogin
		case codeBadRequest:
			var badResp badResp
			if err := json.Unmarshal(bytes, &badResp); err != nil {
				logger.Error("Failed unmarshaling response bytes", zap.Error(err))
				continue
			}
			logger.Error("Bad request", zap.String("message", badResp.Message))
			return nil, ErrBadResponse
		case codeSignError:
			var badResp badResp
			if err := json.Unmarshal(bytes, &badResp); err != nil {
				logger.Error("Failed unmarshaling response bytes", zap.Error(err))
				continue
			}
			logger.Error("Sign error", zap.String("message", badResp.Message))
			return nil, ErrInvalidSign
		case codeOK:
			var okResp okResp
			if err := json.Unmarshal(bytes, &okResp); err != nil {
				logger.Error("Failed unmarshaling response bytes", zap.Error(err))
				continue
			}
			return okResp.Data, nil
		default:
			logger.Error("Unknown response code", zap.Int("code", apiResp.Code))
			return nil, ErrBadResponse
		}
	}

	if err == nil {
		err = ErrMaxRetry
	}
	logger.Error("Failed getting xiaobot response", zap.Error(err))
	return nil, err
}

func (r *RequestService) LimitRaw(u string) (respByte []byte, err error) {
	return nil, nil
}

func (r *RequestService) LimitStream(u string) (resp *http.Response, err error) {
	return nil, nil
}

func (r *RequestService) NoLimit(u string) (respByte []byte, err error) {
	return nil, nil
}

func (r *RequestService) NoLimitStream(u string) (resp *http.Response, err error) {
	return nil, nil
}

// Host:api.xiaobot.net
// Connection:keep-alive
// sec-ch-ua:"Chromium";v="121", "Not A(Brand";v="99"
// app-version:0.1
// DNT:1
// sec-ch-ua-mobile:?0
// Authorization:Bearer 1758768|dAJhxxxxxx8FZVup5tBztopHzMvIsW21zwD6
// User-Agent:Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36
// Accept:application/json, text/plain, */*
// timestamp:1707558382
// api-key:xiaobot_web
// sign:2bb91f1117e3bc7a13e9a688ed7a87b0
// sec-ch-ua-platform:"macOS"
// Origin:https://xiaobot.net
// Sec-Fetch-Site:same-site
// Sec-Fetch-Mode:cors
// Sec-Fetch-Dest:empty
// Referer:https://xiaobot.net/
// Accept-Encoding:gzip, deflate, br (remove this because we do not want to handle gzip)
// Accept-Language:zh-CN,zh;q=0.9,en-US;q=0.8,en;q=0.7
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
