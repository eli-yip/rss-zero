package crawl

import (
	"errors"
	"fmt"
	"slices"

	"github.com/eli-yip/rss-zero/pkg/routers/github/parse"
	"github.com/eli-yip/rss-zero/pkg/routers/github/request"
	"github.com/rs/xid"
	"go.uber.org/zap"
)

func CrawlRepo(user, repo, repoID, token string, parser parse.Parser, logger *zap.Logger) (err error) {
	crawlID := xid.New().String()
	logger = logger.With(zap.String("crawl_id", crawlID))
	logger.Info("Start to crawl github release", zap.String("user", user), zap.String("repo", repo))

	repoToSkip := []string{}

	if slices.Contains(repoToSkip, repo) {
		logger.Warn("Skip this repo by hard-coded slice")
		return nil
	}

	releases, err := request.GetRepoReleases(user, repo, token)
	if err != nil {
		if errors.Is(err, request.ErrNoRelease) {
			logger.Warn("No release found for this repo")
			return nil
		}
		logger.Error("Failed to get github release", zap.Error(err))
		return fmt.Errorf("failed to request github API: %w", err)
	}

	for r := range slices.Values(releases) {
		if err = parser.ParseAndSaveRelease(repoID, r); err != nil {
			logger.Error("Failed to parse and save release", zap.Error(err))
			return fmt.Errorf("failed to parse and save release: %w", err)
		}
	}

	return nil
}
