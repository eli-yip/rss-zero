package douyu

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/rs/xid"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/log"
	"github.com/eli-yip/rss-zero/internal/notify"
	"github.com/eli-yip/rss-zero/internal/redis"
)

func BuildCrawlFunc(notifier notify.Notifier, redis redis.Redis) func() {
	return func() {
		crawl("3484", notifier, redis, log.DefaultLogger)
	}
}

const douyuLiveKey = "douyu:live:%s"

func crawl(roomId string, notifier notify.Notifier, r redis.Redis, logger *zap.Logger) {
	tokenCh := make(chan struct{})
	go func() {
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(2*time.Hour))
		defer cancel()
		defer close(tokenCh)
		for {
			select {
			case <-ctx.Done():
				return
			case tokenCh <- struct{}{}:
				randInt := rand.IntN(5)
				time.Sleep(time.Duration(randInt+5) * time.Minute)
			}
		}
	}()

	for range tokenCh {
		logger := logger.With(zap.String("cron_job_id", xid.New().String()))

		logger.Info("start check douyu room", zap.String("room_id", roomId))

		if v, err := r.Get(fmt.Sprintf(douyuLiveKey, roomId)); err != nil {
			if !errors.Is(err, redis.ErrKeyNotExist) {
				logger.Error("failed to get douyu room live status", zap.Error(err))
				return
			}
		} else if v == "1" {
			logger.Info("douyu room is live, skip check")
			return
		}

		newApiUrl := fmt.Sprintf("https://www.douyu.com/betard/%s", roomId)

		data, err := requestUrl(context.Background(), newApiUrl)
		if err != nil {
			logger.Error("failed to request new api", zap.Error(err))
			return
		}

		info, err := parseBetardInfo(data)
		if err != nil {
			logger.Error("failed to parse betard info", zap.Error(err))
			return
		}

		if info != nil {
			logger.Info("douyu room is live", zap.String("room_id", roomId), zap.Time("start_time", info.startTime))
			notify.NoticeWithLogger(notifier, fmt.Sprintf("[douyu] %s is live", roomId), "", logger)
			r.Set(fmt.Sprintf(douyuLiveKey, roomId), 1, 10*time.Hour) // Use 10 hours as we only check live status in 19:00-20:30
			return
		}

		oldApiUrl := fmt.Sprintf("http://open.douyucdn.cn/api/RoomApi/room/%s", roomId)
		data, err = requestUrl(context.Background(), oldApiUrl, WithReferer(fmt.Sprintf("https://www.douyu.com/%s", roomId)))
		if err != nil {
			logger.Error("failed to request old api", zap.Error(err))
			return
		}

		info, err = parseOldApi(data)
		if err != nil {
			logger.Error("failed to parse old api", zap.Error(err))
			return
		}

		if info != nil {
			logger.Info("douyu room is live", zap.String("room_id", roomId), zap.Time("start_time", info.startTime))
			notify.NoticeWithLogger(notifier, fmt.Sprintf("[douyu] %s is live", roomId), "", logger)
			r.Set(fmt.Sprintf(douyuLiveKey, roomId), "1", 10*time.Hour)
			return
		}

		logger.Info("douyu room is not live", zap.String("room_id", roomId))
	}
}
