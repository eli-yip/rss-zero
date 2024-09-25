package common

import (
	"errors"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/redis"
)

type Task struct {
	TextCh chan string
	ErrCh  chan error
	Logger *zap.Logger
}

type RssGenerator func(string, *zap.Logger) (string, error)

func BuildTaskProcessor(taskCh chan Task, redisService redis.Redis, rssGenerator RssGenerator) func() {
	return func() {
		for task := range taskCh {
			key := <-task.TextCh
			logger := task.Logger

			content, err := redisService.Get(key)
			if err == nil {
				task.TextCh <- content
				logger.Info("Get rss from redis successfully")
				continue
			}

			if errors.Is(err, redis.ErrKeyNotExist) {
				logger.Info("Key does not exist in redis, start to generate rss")
				content, err = rssGenerator(key, logger)
				if err != nil {
					task.ErrCh <- err
					continue
				}
				logger.Info("Generate rss successfully")
				task.TextCh <- content
				continue
			}

			task.ErrCh <- err
		}
	}
}
