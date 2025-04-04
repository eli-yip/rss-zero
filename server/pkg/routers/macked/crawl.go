package macked

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/rs/xid"
	"github.com/samber/lo"
)

var mutex *sync.Mutex

func init() {
	mutex = &sync.Mutex{}
}

func CrawlFunc(redisService redis.Redis, db DB, logger *zap.Logger) func() {
	return func() {
		logger := logger.With(zap.String("cron_job_id", xid.New().String()))
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
		logger.Error("Failed to get latest post time in db", zap.Error(err))
		return fmt.Errorf("failed to get latest post time in db: %w", err)
	}
	logger.Info("Got latest post time in db", zap.Time("latest_post_time", latestPostTimeInDB))

	posts, err := GetLatestPosts()
	if err != nil {
		logger.Error("Failed to get latest posts", zap.Error(err))
		return fmt.Errorf("failed to get latest posts: %w", err)
	}
	logger.Info("Got latest posts", zap.Int("num_of_posts", len(posts)))

	parsedPosts, err := ParsePosts(posts)
	if err != nil {
		logger.Error("Failed to parse posts", zap.Error(err))
		return fmt.Errorf("failed to parse posts: %w", err)
	}
	logger.Info("Parsed posts", zap.Int("num_of_parsed_posts", len(parsedPosts)))

	subscribedAppInfos, err := db.GetAppInfos()
	if err != nil {
		logger.Error("Failed to get subscribed app infos", zap.Error(err))
		return fmt.Errorf("failed to get subscribed app infos: %w", err)
	}
	subscribedAppNames := lo.Map(subscribedAppInfos, func(info AppInfo, _ int) string { return info.AppName })

	var unreadPosts []ParsedPost
	for _, p := range parsedPosts {
		if !p.Modified.After(latestPostTimeInDB) {
			break
		}
		idx := slices.IndexFunc(subscribedAppNames, func(name string) bool { return strings.HasPrefix(strings.ToLower(p.Title), strings.ToLower(name)) })
		if idx == -1 {
			continue
		}
		unreadPosts = append(unreadPosts, p)
		appInfo := subscribedAppInfos[idx]
		logger.Info("Found unread post",
			zap.String("post_title", p.Title),
			zap.String("app_name", appInfo.AppName),
			zap.String("macked_app_id", p.ID),
			zap.String("macked_app_name", p.Title))
		if err = db.UpdateAppInfo(appInfo.ID, p.Modified); err != nil {
			logger.Error("Failed to update app info", zap.Error(err))
			return fmt.Errorf("failed to update app info: %w", err)
		}
		logger.Info("Updated app info", zap.String("app_id", appInfo.ID), zap.String("app_name", appInfo.AppName))
	}

	if err = renderAndSaveRSS(redisService, unreadPosts); err != nil {
		logger.Error("Failed to render and save rss", zap.Error(err))
		return fmt.Errorf("failed to render and save rss: %w", err)
	}
	logger.Info("Rendered and saved rss")

	if len(unreadPosts) == 0 {
		logger.Info("No unread posts, skip saving post time to db")
		return nil
	}

	slices.SortFunc(parsedPosts, func(a, b ParsedPost) int {
		return b.Modified.Compare(a.Modified)
	})
	if err = db.SaveTime(parsedPosts[0].Modified); err != nil {
		logger.Error("Failed to save post time to db", zap.Error(err))
		return fmt.Errorf("failed to save post time to db: %w", err)
	}
	logger.Info("Saved latest post time to db", zap.Time("post_time", parsedPosts[0].Modified))

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
