package crawl

import (
	"fmt"

	"github.com/eli-yip/rss-zero/pkg/routers/github/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/github/request"
	"github.com/rs/xid"
	"go.uber.org/zap"
)

func CrawlRepo(user, repo, subID, token string, parser parse.Parser, logger *zap.Logger) (err error) {
	crawlID := xid.New().String()
	logger = logger.With(zap.String("crawl_id", crawlID))
	logger.Info("Start to crawl github release", zap.String("user", user), zap.String("repo", repo))

	releases, err := request.GetRepoReleases(user, repo, token)
	if err != nil {
		logger.Error("Failed to get github release", zap.Error(err))
		return fmt.Errorf("failed to request github API: %w", err)
	}

	for _, r := range releases {
		if err = parser.ParseAndSaveRelease(subID, r); err != nil {
			logger.Error("Failed to parse and save release", zap.Error(err))
			return fmt.Errorf("failed to parse and save release: %w", err)
		}
	}

	return nil
}
