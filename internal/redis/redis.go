package redis

import (
	"context"
	"errors"
	"time"

	redis "github.com/redis/go-redis/v9"
)

var ErrKeyNotExist = errors.New("key does not exist")

const Forever = 0
const ZsxqCookiePath = "zsxq_cookie"

var RSSTTL = time.Hour * 2

type RedisService struct {
	client *redis.Client
	ctx    context.Context
}

type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

func NewRedisService(c RedisConfig) (service *RedisService, err error) {
	client := redis.NewClient(&redis.Options{
		Addr:     c.Addr,
		Password: c.Password,
		DB:       c.DB,
	})

	service = &RedisService{
		client: client,
		ctx:    context.Background(),
	}

	_, err = service.client.Ping(service.ctx).Result()
	if err != nil {
		return nil, err
	}
	return service, nil
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
