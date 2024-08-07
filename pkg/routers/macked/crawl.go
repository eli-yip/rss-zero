package macked

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/redis"
)

func Crawl(redisService redis.Redis, bot BotIface, db DB, logger *zap.Logger) (err error) {
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

	go func() {
		for i := len(parsedPosts) - 1; i >= 0; i-- {
			if parsedPosts[i].Modified.After(latestPostTimeInDB) {
				if err = db.SaveTime(parsedPosts[i].Modified); err != nil {
					logger.Error("Failed to save post time to db", zap.Error(err))
					return
				}

				text := fmt.Sprintf(`Release: %s
https://macked.app/?p=%s`, parsedPosts[i].Title, parsedPosts[i].ID)

				if err = bot.SendText(config.C.Telegram.MackedChatID, text); err != nil {
					logger.Error("Failed to send message to telegram", zap.Error(err))
					return
				}
			}
		}
	}()

	renderService := NewRSSRenderService()
	rssContent, err := renderService.RenderRSS(parsedPosts)
	if err != nil {
		return fmt.Errorf("fail to render rss content: %w", err)
	}

	if err = redisService.Set(redis.RssMackedPath, rssContent, redis.RSSDefaultTTL); err != nil {
		return fmt.Errorf("fail to set rss content to redis: %w", err)
	}

	return nil
}
