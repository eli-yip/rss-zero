package crawl

import (
	"fmt"

	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/pkg/routers/endoflife"
)

func CrawlEndOfLife(product string, redisService redis.Redis) (err error) {
	cycles, err := endoflife.GetReleaseCycles(product)
	if err != nil {
		return fmt.Errorf("fail to get release cycles from endoflife: %w", err)
	}

	versionInfoList, err := endoflife.ParseCycles(cycles)
	if err != nil {
		return fmt.Errorf("fail to parse release cycles: %w", err)
	}

	renderService := endoflife.NewRSSRenderService()
	rssContent, err := renderService.RenderRSS(product, versionInfoList)
	if err != nil {
		return fmt.Errorf("fail to render rss content: %w", err)
	}

	path := fmt.Sprintf(redis.EndOfLifePath, product)

	if err = redisService.Set(path, rssContent, redis.DefaultTTL); err != nil {
		return fmt.Errorf("fail to set rss content to redis: %w", err)
	}

	return nil
}
