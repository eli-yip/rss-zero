package controller

import (
	"errors"
	"net/http"
	"slices"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/eli-yip/rss-zero/pkg/routers/github/db"
)

type filterConfig struct {
	SubID      []string
	Prerelease bool
	Deleted    bool
}

func (h *Controller) GetSubs(c *echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	subs, err := h.db.GetSubs()
	if err != nil {
		logger.Error("Failed to get github sub list", zap.Error(err))
		return httputil.NewHTTPError(http.StatusInternalServerError, "Failed to get github sub list")
	}
	logger.Info("Get zhihu sub list successfully", zap.Int("count", len(subs)))

	filterConfig, err := parseFilterConfig(c)
	if err != nil {
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	logger.Info("Filter config", zap.Any("config", filterConfig))

	filteredSubs, err := filterSubs(subs, filterConfig)
	if err != nil {
		logger.Error("Failed to filter subs", zap.Error(err))
		return httputil.NewHTTPError(http.StatusInternalServerError, "Failed to filter subs")
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
			return httputil.NewHTTPError(http.StatusInternalServerError, "Failed to get repo info")
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

	return c.JSON(http.StatusOK, httputil.NewResp("success", resp))
}

func parseFilterConfig(c *echo.Context) (filterConfig filterConfig, err error) {
	filterConfig.SubID, err = echo.QueryParamsOr[string](c, "sub_id", nil)
	if err != nil {
		return filterConfig, err
	}
	filterConfig.SubID = common.RemoveEmptyStringInStringSlice(filterConfig.SubID)
	prerelease, err := echo.QueryParamOr[string](c, "prerelease", "")
	if err != nil {
		return filterConfig, err
	}
	filterConfig.Prerelease = prerelease == "true"
	if _, deletedErr := echo.QueryParam[string](c, "deleted"); deletedErr == nil {
		filterConfig.Deleted = true
	} else if !errors.Is(deletedErr, echo.ErrNonExistentKey) {
		return filterConfig, deletedErr
	}
	return filterConfig, nil
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

func (h *Controller) ActivateSub(c *echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	id, err := echo.PathParam[string](c, "id")
	if err != nil {
		return httputil.NewHTTPError(http.StatusBadRequest, "missing subscription ID")
	}
	logger.Info("Start to activate github sub", zap.String("id", id))

	if err = h.db.ActivateSub(id); err != nil {
		logger.Error("Failed to activate github sub", zap.Error(err))
		return httputil.NewHTTPError(http.StatusInternalServerError, "Failed to activate github sub")
	}
	return c.JSON(http.StatusOK, httputil.NewMessage("Success"))
}

func (h *Controller) DeleteSub(c *echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	id, err := echo.PathParam[string](c, "id")
	if err != nil {
		return httputil.NewHTTPError(http.StatusBadRequest, "missing subscription ID")
	}
	logger.Info("Start to delete github sub", zap.String("id", id))

	if err = h.db.DeleteSub(id); err != nil {
		logger.Error("Failed to delete github sub", zap.Error(err))
		return httputil.NewHTTPError(http.StatusInternalServerError, "Failed to delete github sub")
	}
	return c.JSON(http.StatusOK, httputil.NewMessage("Success"))
}
