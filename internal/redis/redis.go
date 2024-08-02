package redis

import (
	"context"
	"errors"
	"time"

	"github.com/eli-yip/rss-zero/config"
	redis "github.com/redis/go-redis/v9"
)

var ErrKeyNotExist = errors.New("key does not exist")

const (
	ZsxqRSSPath = "zsxq_rss_%s"

	XiaobotRSSPath = "xiaobot_rss_%s"

	ZhihuAnswerPath  = "zhihu_rss_answer_%s"
	ZhihuArticlePath = "zhihu_rss_article_%s"
	ZhihuPinPath     = "zhihu_rss_pin_%s"

	EndOfLifePath = "endoflife_rss_%s"

	GitHubRSSPath = "github_rss_%s"
)

const (
	ZSECKTTL      = 24 * 2 * time.Hour
	RSSDefaultTTL = time.Hour * 2
)

type Redis interface {
	Set(key string, value interface{}, duration time.Duration) (err error)
	Get(key string) (value string, err error)
	Del(key string) (err error)
	TTL(key string) (time.Duration, error)
}

type RedisService struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisService(c config.RedisConfig) (service Redis, err error) {
	client := redis.NewClient(&redis.Options{
		Addr:     c.Address,
		Password: c.Password,
		DB:       0,
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

func (s *RedisService) TTL(key string) (ttl time.Duration, err error) {
	ttl, err = s.client.TTL(s.ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, ErrKeyNotExist
		}
	}
	return ttl, nil
}
