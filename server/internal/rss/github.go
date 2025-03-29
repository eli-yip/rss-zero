package rss

import (
	"fmt"
	"slices"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/redis"
	githubDB "github.com/eli-yip/rss-zero/pkg/routers/github/db"
	render "github.com/eli-yip/rss-zero/pkg/routers/github/render"
)

func GenerateGitHub(subID string, dbService githubDB.DB, logger *zap.Logger) (path, content string, err error) {
	logger.Info("Start to generate github release rss")
	rssRender := render.NewRSSRenderService()
	logger.Info("Init github rss render service successfully")

	sub, err := dbService.GetSubByIDIncludeDeleted(subID)
	if err != nil {
		return "", "", fmt.Errorf("failed to get sub info from database: %w", err)
	}
	logger.Info("Get sub info successfully")

	repo, err := dbService.GetRepoByID(sub.RepoID)
	if err != nil {
		return "", "", fmt.Errorf("failed to get repo info from database: %w", err)
	}
	logger.Info("Get repo info successfully")

	path = fmt.Sprintf(redis.GitHubRSSPath, subID)

	releases, err := dbService.GetReleases(repo.ID, sub.PreRelease, 1, 20)
	if err != nil {
		return "", "", fmt.Errorf("failed to get releases from database: %w", err)
	}
	if len(releases) == 0 {
		logger.Info("Found no release in database, render empty content now")
		content, err = rssRender.RenderEmpty(repo.GithubUser, repo.Name, sub.PreRelease)
		return path, content, err
	}

	rs := make([]render.RSSItem, 0, len(releases))
	for r := range slices.Values(releases) {
		rs = append(rs, render.RSSItem{
			ID:         r.ID,
			Link:       r.URL,
			UpdateTime: r.PublishedAt,
			RepoName:   repo.Name,
			Title: func() string {
				if r.Title == "" {
					return r.Tag
				}
				return r.Title
			}(),
			Body: func() string {
				if r.Body == "" {
					return r.RawBody
				}
				return r.Body
			}(),
			TagName:  r.Tag,
			Prelease: r.PreRelease,
		})
	}
	logger.Info("Generate rss items successfully")

	content, err = rssRender.Render(rs, sub.PreRelease)
	if err != nil {
		return "", "", fmt.Errorf("failed to render rss: %w", err)
	}

	return path, content, nil
}
