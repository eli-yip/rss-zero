package controller

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	pkgCommon "github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
)

type filterConfig struct {
	AuthorID    []string
	SubID       []string
	ContentType []string
	Deleted     bool
}

func (h *Controller) GetSubs(c echo.Context) error {
	logger := common.ExtractLogger(c)

	subs, err := h.db.GetSubsIncludeDeleted()
	if err != nil {
		logger.Error("Failed to get zhihu sub list", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.ApiResp{Message: "Failed to get zhihu sub list"})
	}
	logger.Info("Get zhihu sub list successfully", zap.Int("count", len(subs)))

	var filterConfig filterConfig
	params := c.QueryParams()
	if params.Has("author") {
		filterConfig.AuthorID = params["author"]
		filterConfig.AuthorID = common.RemoveEmptyStringInStringSlice(filterConfig.AuthorID)
	}
	if params.Has("sub_id") {
		filterConfig.SubID = params["sub_id"]
		filterConfig.SubID = common.RemoveEmptyStringInStringSlice(filterConfig.SubID)
	}
	if params.Has("type") {
		filterConfig.ContentType = params["type"]
		filterConfig.ContentType = common.RemoveEmptyStringInStringSlice(filterConfig.ContentType)
	}
	if params.Has("deleted") {
		filterConfig.Deleted = true
	}
	logger.Info("Filter config", zap.Any("config", filterConfig))

	filteredSubs, err := filterSubs(subs, filterConfig)
	if err != nil {
		logger.Error("Failed to filter subs", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.ApiResp{Message: "Failed to filter subs"})
	}
	logger.Info("Filter subs successfully", zap.Int("count", len(filteredSubs)))

	type (
		SingleSubInfo struct {
			SubType string `json:"sub_type"`
			Url     string `json:"url"`
			Deleted bool   `json:"deleted"`
		}
		SubInfo struct {
			Nickname  string          `json:"nickname"`
			SignleSub []SingleSubInfo `json:"single_sub"`
		}
		Sub  map[string]SubInfo
		Resp struct {
			Subs Sub `json:"subs"`
		}
	)

	nicknameMap := make(map[string]string)
	respMap := make(Sub)

	for _, sub := range filteredSubs {
		nickname, ok := nicknameMap[sub.AuthorID]
		if !ok {
			nickname, err = h.db.GetAuthorName(sub.AuthorID)
			if err != nil {
				logger.Error("Failed to get author name", zap.String("author_id", sub.AuthorID), zap.Error(err))
				return c.JSON(http.StatusInternalServerError, common.ApiResp{Message: "Failed to get author name"})
			}
			nicknameMap[sub.AuthorID] = nickname
		}

		singleSub := SingleSubInfo{
			SubType: pkgCommon.ZhihuTypeToString(sub.Type),
			Url:     fmt.Sprintf("https://www.zhihu.com/people/%s/%s", sub.AuthorID, pkgCommon.ZhihuTypeToLinkType(sub.Type)),
			Deleted: sub.DeletedAt.Valid,
		}

		subInfo, ok := respMap[nickname]
		if !ok {
			subInfo = SubInfo{
				Nickname:  nickname,
				SignleSub: make([]SingleSubInfo, 0),
			}
		}
		subInfo.SignleSub = append(subInfo.SignleSub, singleSub)
		respMap[sub.AuthorID] = subInfo
	}

	return c.JSON(http.StatusOK, &Resp{Subs: respMap})
}

func filterSubs(subs []db.Sub, config filterConfig) ([]db.Sub, error) {
	filteredSubs := make([]db.Sub, 0)

	// If deleted is true, subs to filter are the ones that are deleted
	if config.Deleted {
		subsToFilter := make([]db.Sub, 0)
		for _, sub := range subs {
			if sub.DeletedAt.Valid {
				subsToFilter = append(subsToFilter, sub)
			}
		}
		subs = subsToFilter
	}

	if len(config.AuthorID) == 0 && len(config.SubID) == 0 {
		if len(config.ContentType) == 0 {
			return subs, nil
		}

		for _, sub := range subs {
			if slices.Contains(config.ContentType, pkgCommon.ZhihuTypeToString(sub.Type)) {
				filteredSubs = append(filteredSubs, sub)
			}
		}
		return filteredSubs, nil
	}

	for _, sub := range subs {
		if (slices.Contains(config.AuthorID, sub.AuthorID) ||
			slices.Contains(config.SubID, sub.ID)) &&
			slices.Contains(config.ContentType, pkgCommon.ZhihuTypeToString(sub.Type)) {
			filteredSubs = append(filteredSubs, sub)
		}
	}

	return filteredSubs, nil
}

func (h *Controller) ActivateSub(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	id := c.Param("id")
	logger.Info("Start to activate zhihu sub", zap.String("id", id))

	if err = h.db.ActivateSub(id); err != nil {
		logger.Error("Failed to activate zhihu sub", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.ApiResp{Message: "Failed to activate zhihu sub"})
	}
	return c.JSON(http.StatusOK, common.ApiResp{Message: "Success"})
}

func (h *Controller) DeleteSub(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	id := c.Param("id")
	logger.Info("Start to delete zhihu sub", zap.String("id", id))

	if err = h.db.DeleteSub(id); err != nil {
		logger.Error("Failed to delete zhihu sub", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.ApiResp{Message: "Failed to delete zhihu sub"})
	}
	return c.JSON(http.StatusOK, common.ApiResp{Message: "Success"})
}
