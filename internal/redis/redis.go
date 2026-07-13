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
	// v2 namespace（plan 决策 7）：删 text 列后读取期从 raw 重放正文，换 key 隔离旧 text 生成的
	// 陈旧 canonical items。cron warm 与 controller cache-miss 共用同一 const，一并切换。
	ZsxqRSSPath                  = "zsxq_rss_v2_%s"
	ZsxqRandomCanglimoDigestPath = "zsxq_rss_random_canglimo_digest_v2"

	XiaobotRSSPath = "xiaobot_rss_%s"

	// v2 namespace（plan 决策 6）：删 text 列后随机答案正文改从 raw 重放，换 key 隔离旧 text
	// 生成的陈旧 items。cron warm（cron/random.go）与 controller serve（controller/zhihu/random.go）
	// 共用同一 const，一并切换。每作者 feed 的 v2 键见 common.ZhihuContentType.RedisKey。
	ZhihuRandomCanglimoAnswersPath = "zhihu_rss_random_canglimo_answers_v2"

	EndOfLifePath = "endoflife_rss_%s"

	GitHubRSSPath = "github_rss_%s"

	RssMackedPath = "macked_rss"

	RssTombkeeperTimelinePath = "tombkeeper_timeline_rss"
)

const (
	ZSECKTTL      = 24 * 2 * time.Hour
	RSSDefaultTTL = time.Hour * 2
	RSSRandomTTL  = time.Hour * 24
)

type Redis interface {
	Set(key string, value any, duration time.Duration) (err error)
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

func (s *RedisService) Set(key string, value any, duration time.Duration) (err error) {
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
