package endoflife

import (
	"fmt"

	"github.com/eli-yip/rss-zero/internal/redis"
)

func Crawl(product string, redisService redis.Redis) (err error) {
	cycles, err := GetReleaseCycles(product)
	if err != nil {
		return fmt.Errorf("failed to get release cycles from endoflife: %w", err)
	}

	versionInfoList, err := ParseCycles(cycles)
	if err != nil {
		return fmt.Errorf("failed to parse release cycles: %w", err)
	}

	renderService := NewRSSRenderService()
	rssContent, err := renderService.RenderRSS(product, versionInfoList)
	if err != nil {
		return fmt.Errorf("failed to render rss content: %w", err)
	}

	path := fmt.Sprintf(redis.EndOfLifePath, product)

	if err = redisService.Set(path, rssContent, redis.RSSDefaultTTL); err != nil {
		return fmt.Errorf("failed to set rss content to redis: %w", err)
	}

	return nil
}
