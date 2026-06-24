package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/rs/xid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	githubDB "github.com/eli-yip/rss-zero/pkg/routers/github/db"
)

// RSS serves a github release feed through the unified pipeline. checkRepo (with
// the /rss/github/pre prefix selecting pre-releases) resolves/creates the
// subscription before the generic Serve fetches and renders.
func (h *Controller) RSS(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	feed := c.Get("feed_id").(string)
	userRepo := strings.Split(feed, "/")
	if len(userRepo) != 2 {
		logger.Error("Error getting user and repo, length not equal to 2", zap.String("feed", feed))
		return httputil.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	user, repo := userRepo[0], userRepo[1]

	pre := strings.HasPrefix(c.Request().URL.Path, "/rss/github/pre")

	subID, err := h.checkRepo(user, repo, pre)
	if err != nil {
		if errors.Is(err, ErrRepoNotFound) {
			logger.Error("Error return rss", zap.String("user", user), zap.String("repo", repo), zap.Error(err))
			return c.String(http.StatusBadRequest, "repo not found")
		}
		logger.Error("Failed to GitHub repo", zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to check repo")
	}
	logger.Info("Check repo successfully")

	return rss.Serve(c, rss.ServeOptions{
		Redis:        h.redis,
		Logger:       logger,
		Key:          fmt.Sprintf(redis.GitHubRSSPath, subID),
		TTL:          redis.RSSDefaultTTL,
		DefaultLimit: 20,
		Fetch: func() (rss.FeedMeta, []rss.Item, error) {
			return rss.FetchGitHub(subID, h.db, logger)
		},
	})
}

var ErrRepoNotFound = errors.New("repo not found")

func (h *Controller) checkRepo(user, repoName string, pre bool) (subID string, err error) {
	var repoID string

	var repo *githubDB.Repo
	if repo, err = h.db.GetRepo(user, repoName); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			resp, err := http.Get(fmt.Sprintf("https://github.com/%s/%s", user, repoName))
			if err != nil {
				return "", fmt.Errorf("failed to request github normal page")
			}

			if resp.StatusCode != http.StatusOK {
				return "", ErrRepoNotFound
			}

			repoID = xid.New().String()
			if err = h.db.SaveRepo(&githubDB.Repo{
				ID:         repoID,
				GithubUser: user,
				Name:       repoName,
			}); err != nil {
				return "", fmt.Errorf("failed to save repo: %w", err)
			}
		} else {
			return "", fmt.Errorf("failed to get repo: %w", err)
		}
	} else {
		repoID = repo.ID
	}

	sub, err := h.db.GetSubIncludeDeleted(repoID, pre)
	if err == nil {
		return sub.ID, nil
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", fmt.Errorf("failed to get sub: %w", err)
	}

	subID = xid.New().String()
	err = h.db.SaveSub(&githubDB.Sub{
		ID:         subID,
		RepoID:     repoID,
		PreRelease: pre,
	})
	return subID, err
}
