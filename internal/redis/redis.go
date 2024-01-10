package redis

import (
	"context"
	"errors"
	"time"

	redis "github.com/redis/go-redis/v9"
)

var ErrKeyNotExist = errors.New("key does not exist")

type RedisService struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisService(addr, password string, db int) (service *RedisService) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	service = &RedisService{
		client: client,
		ctx:    context.Background(),
	}

	_, err := service.client.Ping(service.ctx).Result()
	if err != nil {
		panic(err) // TODO: Handle error with zap.
	}
	return nil
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
