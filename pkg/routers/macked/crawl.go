package macked

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/redis"
)

var mutex *sync.Mutex

func init() {
	mutex = &sync.Mutex{}
}

func CrawlFunc(redisService redis.Redis, bot BotIface, db DB, logger *zap.Logger) func() {
	return func() {
		if err := Crawl(redisService, bot, db, logger); err != nil {
			logger.Error("Failed to crawl macked", zap.Error(err))
		}
	}
}

func Crawl(redisService redis.Redis, bot BotIface, db DB, logger *zap.Logger) (err error) {
	mutex.Lock()
	defer mutex.Unlock()

	latestPostTimeInDB, err := db.GetLatestTime()
	if err != nil {
		return fmt.Errorf("fail to get latest post time in db: %w", err)
	}

	posts, err := GetLatestPosts()
	if err != nil {
		return fmt.Errorf("fail to get latest posts: %w", err)
	}

	parsedPosts, err := ParsePosts(posts)
	if err != nil {
		return fmt.Errorf("fail to parse posts: %w", err)
	}

	var unreadPosts []ParsedPost
	for _, p := range parsedPosts {
		if !p.Modified.After(latestPostTimeInDB) {
			break
		}
		unreadPosts = append(unreadPosts, p)
	}

	slices.Reverse(unreadPosts) // Reverse unread posts because we want to notify in tg channel from old to latest
	var count int = 0
	go func() {
		for _, p := range unreadPosts {
			if count >= 10 {
				logger.Info("Reach telegram bot limit, sleep 30 seconds")
				time.Sleep(30 * time.Second)
				count = 0
			}

			if err = db.SaveTime(p.Modified); err != nil {
				logger.Error("Failed to save post time to db", zap.Error(err))
				return
			}

			text := fmt.Sprintf(`%s
%s`, p.Title, p.Link)

			if err = bot.SendText(config.C.Telegram.MackedChatID, text); err != nil {
				logger.Error("Failed to send message to telegram", zap.Error(err))
				return
			}

			count++
		}
	}()

	renderService := NewRSSRenderService()
	rssContent, err := renderService.RenderRSS(unreadPosts)
	if err != nil {
		return fmt.Errorf("fail to render rss content: %w", err)
	}

	if err = redisService.Set(redis.RssMackedPath, rssContent, redis.RSSDefaultTTL); err != nil {
		return fmt.Errorf("fail to set rss content to redis: %w", err)
	}

	return nil
}
