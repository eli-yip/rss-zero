package macked

import (
	"fmt"
	"slices"
	"sync"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/samber/lo"
)

var mutex *sync.Mutex

func init() {
	mutex = &sync.Mutex{}
}

func CrawlFunc(redisService redis.Redis, db DB, logger *zap.Logger) func() {
	return func() {
		if err := Crawl(redisService, db, logger); err != nil {
			logger.Error("Failed to crawl macked", zap.Error(err))
		}
	}
}

func Crawl(redisService redis.Redis, db DB, logger *zap.Logger) (err error) {
	mutex.Lock()
	defer mutex.Unlock()

	latestPostTimeInDB, err := db.GetLatestTime()
	if err != nil {
		return fmt.Errorf("failed to get latest post time in db: %w", err)
	}

	posts, err := GetLatestPosts()
	if err != nil {
		return fmt.Errorf("failed to get latest posts: %w", err)
	}

	parsedPosts, err := ParsePosts(posts)
	if err != nil {
		return fmt.Errorf("failed to parse posts: %w", err)
	}

	subscribedAppInfos, err := db.GetAppInfos()
	if err != nil {
		return fmt.Errorf("failed to get subscribed app infos: %w", err)
	}
	subscribedAppNames := lo.Map(subscribedAppInfos, func(info AppInfo, _ int) string { return info.AppName })

	var unreadPosts []ParsedPost
	for _, p := range parsedPosts {
		if !p.Modified.After(latestPostTimeInDB) {
			break
		}
		if isSubscribed(p.Title, subscribedAppNames) {
			unreadPosts = append(unreadPosts, p)
		}
	}

	if err = renderAndSaveRSS(redisService, unreadPosts); err != nil {
		return fmt.Errorf("failed to render and save rss: %w", err)
	}

	if len(unreadPosts) == 0 {
		return nil
	}

	slices.Reverse(unreadPosts) // Reverse unread posts because we want to notify in tg channel from old to latest
	for _, p := range unreadPosts {
		if err = db.SaveTime(p.Modified); err != nil {
			logger.Error("Failed to save post time to db", zap.Error(err))
			return fmt.Errorf("failed to save post time to db: %w", err)
		}
	}

	return nil
}

func renderAndSaveRSS(redisService redis.Redis, posts []ParsedPost) (err error) {
	var rssContent string
	renderService := NewRSSRenderService()
	if len(posts) == 0 {
		rssContent, err = renderService.RenderEmptyRSS()
		if err != nil {
			return fmt.Errorf("failed to render empty rss content: %w", err)
		}
	} else {
		rssContent, err = renderService.RenderRSS(posts)
		if err != nil {
			return fmt.Errorf("failed to render rss content: %w", err)
		}
	}

	if err = redisService.Set(redis.RssMackedPath, rssContent, redis.RSSDefaultTTL); err != nil {
		return fmt.Errorf("failed to set rss content to redis: %w", err)
	}

	return nil
}
