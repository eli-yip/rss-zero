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

	"github.com/eli-yip/rss-zero/cmd/server/controller/common"
	"github.com/eli-yip/rss-zero/internal/redis"
	"github.com/eli-yip/rss-zero/internal/rss"
	githubDB "github.com/eli-yip/rss-zero/pkg/routers/github/db"
)

func (h *Controller) RSS(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	feed := c.Get("feed_id").(string)
	userRepo := strings.Split(feed, "/")
	if len(userRepo) != 2 {
		logger.Error("Error getting user and repo, length not equal to 2", zap.String("feed", feed))
		return c.JSON(http.StatusBadRequest, &common.ApiResp{Message: "invalid request"})
	}
	user, repo := userRepo[0], userRepo[1]

	var pre bool
	path := c.Request().URL.Path
	if strings.HasPrefix(path, "/rss/github/pre") {
		pre = true
	}

	var subID string
	if subID, err = h.checkRepo(user, repo, pre); err != nil {
		if errors.Is(err, ErrRepoNotFound) {
			logger.Error("Error return rss", zap.String("user", user), zap.String("repo", repo), zap.Error(err))
			return c.String(http.StatusBadRequest, "repo not found")
		}
		logger.Error("Failed checking repo", zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to check repo")
	}
	logger.Info("Check repo successfully")

	rss, err := h.getRSS(fmt.Sprintf(redis.GitHubRSSPath, subID), logger)
	if err != nil {
		logger.Error("Failed to get rss from redis", zap.Error(err))
		return c.String(http.StatusInternalServerError, "failed to get rss from redis")
	}
	logger.Info("Retrieved rss from redis")

	return c.String(http.StatusOK, rss)
}

func (h *Controller) getRSS(key string, logger *zap.Logger) (rssContent string, err error) {
	logger = logger.With(zap.String("rss_path", key))
	defer logger.Info("Task channel closes")

	task := common.Task{TextCh: make(chan string), ErrCh: make(chan error), Logger: logger}
	defer close(task.TextCh)
	defer close(task.ErrCh)

	h.taskCh <- task
	task.TextCh <- key
	logger.Info("Task sent to task channel")

	select {
	case rssContent := <-task.TextCh:
		return rssContent, nil
	case err := <-task.ErrCh:
		return "", err
	}
}

func (h *Controller) processTask() {
	for task := range h.taskCh {
		key := <-task.TextCh

		content, err := h.redis.Get(key)
		if err == nil {
			task.TextCh <- content
			continue
		}

		if errors.Is(err, redis.ErrKeyNotExist) {
			content, err = h.generateRSS(key, task.Logger)
			if err != nil {
				task.ErrCh <- err
				continue
			}
			task.TextCh <- content
			continue
		}

		task.ErrCh <- err
		continue
	}
}

func (h *Controller) generateRSS(key string, logger *zap.Logger) (rssContent string, err error) {
	id := strings.TrimPrefix(key, fmt.Sprintf(redis.GitHubRSSPath, ""))

	_, content, err := rss.GenerateGitHub(id, h.db, logger)
	if err != nil {
		return "", fmt.Errorf("failed to generate rss: %w", err)
	}

	if err = h.redis.Set(key, content, redis.RSSDefaultTTL); err != nil {
		return "", fmt.Errorf("failed to set rss: %w", err)
	}

	return content, nil
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

	sub, err := h.db.GetSub(repoID, pre)
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
