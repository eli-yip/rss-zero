package macked

import (
	"fmt"

	"github.com/eli-yip/rss-zero/internal/redis"
)

func Crawl(redisService redis.Redis) (err error) {
	posts, err := GetLatestPosts()
	if err != nil {
		return fmt.Errorf("fail to get latest posts: %w", err)
	}

	parsedPosts, err := ParsePosts(posts)
	if err != nil {
		return fmt.Errorf("fail to parse posts: %w", err)
	}

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
