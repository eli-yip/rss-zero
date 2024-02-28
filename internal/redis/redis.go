package redis

import (
	"context"
	"errors"
	"time"

	redis "github.com/redis/go-redis/v9"
)

var ErrKeyNotExist = errors.New("key does not exist")

const (
	Forever    = 0
	DefaultTTL = time.Hour * 2
)
const ZsxqCookiePath = "zsxq_cookie"
const XiaobotTokenPath = "xiaobot_token"

const (
	XiaobotRSSPath = "xiaobot_rss_%s"

	ZhihuAnswerPath  = "zhihu_rss_answer_%s"
	ZhihuArticlePath = "zhihu_rss_article_%s"
	ZhihuPinPath     = "zhihu_rss_pin_%s"
)

var RSSTTL = time.Hour * 2

type Redis interface {
	Set(key string, value interface{}, duration time.Duration) (err error)
	Get(key string) (value string, err error)
	Del(key string) (err error)
}

type RedisService struct {
	client *redis.Client
	ctx    context.Context
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

func NewRedisService(c RedisConfig) (service Redis, err error) {
	client := redis.NewClient(&redis.Options{
		Addr:     c.Addr,
		Password: c.Password,
		DB:       c.DB,
	})

	s := &RedisService{
		client: client,
		ctx:    context.Background(),
	}

	_, err = s.client.Ping(s.ctx).Result()
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (s *RedisService) Set(key string, value interface{}, duration time.Duration) (err error) {
	_, err = s.client.Set(s.ctx, key, value, duration).Result()
	return
}

func (s *RedisService) Get(key string) (value string, err error) {
	value, err = s.client.Get(s.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", ErrKeyNotExist
		}
		return "", errors.New("get key failed")
	}
	return value, nil
}

func (s *RedisService) Del(key string) (err error) {
	_, err = s.client.Del(s.ctx, key).Result()
	if err != nil {
		return errors.New("delete key failed")
	}
	return nil
}
