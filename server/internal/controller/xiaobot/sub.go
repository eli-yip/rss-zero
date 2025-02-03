package controller

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/controller/common"
)

func (h *Controller) GetSubs(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	subs, err := h.db.GetPapersIncludeDeleted()
	if err != nil {
		logger.Error("Failed to get xiaobot sub list", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.ApiResp{Message: "Failed to get xiaobot sub list"})
	}
	logger.Info("Get xiaobot sub list successfully", zap.Int("count", len(subs)))

	type (
		SingleSubInfo struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Deleted bool   `json:"deleted"`
		}
		Resp []SingleSubInfo
	)

	var resp Resp
	for _, sub := range subs {
		resp = append(resp, SingleSubInfo{
			ID:      sub.ID,
			Name:    sub.Name,
			Deleted: sub.DeletedAt.Valid,
		})
	}

	return c.JSON(http.StatusOK, resp)
}

func (h *Controller) ActivateSub(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	subID := c.Param("id")
	logger.Info("Activate xiaobot sub", zap.String("sub_id", subID))

	err = h.db.ActivatePaper(subID)
	if err != nil {
		logger.Error("Failed to activate xiaobot sub", zap.String("sub_id", subID), zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.ApiResp{Message: "Failed to activate xiaobot sub"})
	}
	logger.Info("Activate xiaobot sub successfully", zap.String("sub_id", subID))

	return c.JSON(http.StatusOK, common.ApiResp{Message: "Activate xiaobot sub successfully"})
}

func (h *Controller) DeleteSub(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	subID := c.Param("id")
	logger.Info("Delete xiaobot sub", zap.String("sub_id", subID))

	if err = h.db.DeletePaper(subID); err != nil {
		logger.Error("Failed to delete xiaobot sub", zap.String("sub_id", subID), zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.ApiResp{Message: "Failed to delete xiaobot sub"})
	}
	return c.JSON(http.StatusOK, common.ApiResp{Message: "Delete xiaobot sub successfully"})
}
