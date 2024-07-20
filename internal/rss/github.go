package rss

import (
	"fmt"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/redis"
	githubDB "github.com/eli-yip/rss-zero/pkg/routers/github/db"
	render "github.com/eli-yip/rss-zero/pkg/routers/github/render"
)

func GenerateGitHub(id string, preRelease bool, dbService githubDB.DB, logger *zap.Logger) (path, content string, err error) {
	logger.Info("Start to generate github release rss")
	rssRender := render.NewRSSRenderService()
	logger.Info("Init github rss render service successfully")

	sub, err := dbService.GetSubByID(id)
	if err != nil {
		return "", "", fmt.Errorf("failed to get sub info from database: %w", err)
	}

	path = fmt.Sprintf(redis.GitHubRSSPath, sub.User, sub.Repo)

	releases, err := dbService.GetReleases(id, preRelease, 1, 20)
	if err != nil {
		return "", "", fmt.Errorf("failed to get releases from database: %w", err)
	}
	if len(releases) == 0 {
		logger.Info("Found no release in database, render empty content now")
		content, err = rssRender.RenderEmpty(sub.User, sub.Repo)
		return path, content, err
	}

	rs := make([]render.RSSItem, 0, len(releases))
	for _, r := range releases {
		rs = append(rs, render.RSSItem{
			ID:         r.ID,
			Link:       r.URL,
			UpdateTime: r.PublishedAt,
			RepoName:   sub.Repo,
			Title:      r.Title,
			Body:       r.Body,
			Prelease:   r.PreRelease,
		})
	}
	logger.Info("Generate rss items successfully")

	content, err = rssRender.Render(rs)
	if err != nil {
		return "", "", fmt.Errorf("failed to render rss: %w", err)
	}

	return path, content, nil
}
