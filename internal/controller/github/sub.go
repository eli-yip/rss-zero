package controller

import (
	"net/http"
	"slices"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/pkg/routers/github/db"
)

type filterConfig struct {
	SubID      []string
	Prerelease bool
	Deleted    bool
}

func (h *Controller) GetSubs(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	subs, err := h.db.GetSubs()
	if err != nil {
		logger.Error("Failed to get github sub list", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp("Failed to get github sub list"))
	}
	logger.Info("Get zhihu sub list successfully", zap.Int("count", len(subs)))

	var filterConfig filterConfig
	params := c.QueryParams()
	if params.Has("sub_id") {
		filterConfig.SubID = params["sub_id"]
		filterConfig.SubID = common.RemoveEmptyStringInStringSlice(filterConfig.SubID)
	}
	if params.Has("prerelease") {
		if params.Get("prerelease") == "true" {
			filterConfig.Prerelease = true
		} else {
			filterConfig.Prerelease = false
		}
	}
	if params.Has("deleted") {
		filterConfig.Deleted = true
	}
	logger.Info("Filter config", zap.Any("config", filterConfig))

	filteredSubs, err := filterSubs(subs, filterConfig)
	if err != nil {
		logger.Error("Failed to filter subs", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp("Failed to filter subs"))
	}
	logger.Info("Filter subs successfully", zap.Int("count", len(filteredSubs)))

	type (
		Repo struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			URL  string `json:"url"`
		}
		SingleSubInfo struct {
			ID         string `json:"id"`
			Prerelease bool   `json:"prerelease"`
			Repo       Repo   `json:"repo"`
			Deleted    bool   `json:"deleted"`
		}
		Resp []SingleSubInfo
	)

	resp := make(Resp, 0, len(filteredSubs))
	for _, sub := range filteredSubs {
		repo, err := h.db.GetRepoByID(sub.RepoID)
		if err != nil {
			logger.Error("Failed to get repo", zap.String("repo_id", sub.RepoID), zap.Error(err))
			return c.JSON(http.StatusInternalServerError, common.WrapResp("Failed to get repo info"))
		}

		resp = append(resp, SingleSubInfo{
			ID:         sub.ID,
			Prerelease: sub.PreRelease,
			Repo: Repo{
				ID:   repo.ID,
				Name: repo.GithubUser + `/` + repo.Name,
				URL:  `https://github.com/` + repo.GithubUser + `/` + repo.Name,
			},
			Deleted: sub.DeletedAt.Valid,
		})
	}

	return c.JSON(http.StatusOK, resp)
}

func filterSubs(subs []db.Sub, config filterConfig) ([]db.Sub, error) {
	filteredSubs := make([]db.Sub, 0, len(subs))

	if config.Deleted {
		subsToFilter := make([]db.Sub, 0, len(subs))
		for _, sub := range subs {
			if sub.DeletedAt.Valid {
				subsToFilter = append(subsToFilter, sub)
			}
		}

		subs = subsToFilter
	}

	if len(config.SubID) == 0 && !config.Prerelease {
		return subs, nil
	}

	for _, sub := range subs {
		// If sub id slice is not empty, and current sub id is not in the slice, skip
		if len(config.SubID) != 0 && !slices.Contains(config.SubID, sub.ID) {
			continue
		}
		// If config.Prerelease is true, and current sub is not a prerelease sub, skip
		if config.Prerelease && !sub.PreRelease {
			continue
		}

		filteredSubs = append(filteredSubs, sub)
	}

	return filteredSubs, nil
}

func (h *Controller) ActivateSub(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	id := c.Param("id")
	logger.Info("Start to activate github sub", zap.String("id", id))

	if err = h.db.ActivateSub(id); err != nil {
		logger.Error("Failed to activate github sub", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp("Failed to activate github sub"))
	}
	return c.JSON(http.StatusOK, common.WrapResp("Success"))
}

func (h *Controller) DeleteSub(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	id := c.Param("id")
	logger.Info("Start to delete github sub", zap.String("id", id))

	if err = h.db.DeleteSub(id); err != nil {
		logger.Error("Failed to delete github sub", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp("Failed to delete github sub"))
	}
	return c.JSON(http.StatusOK, common.WrapResp("Success"))
}
