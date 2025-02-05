package archive

import (
	"fmt"
	"net/http"
	"time"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/controller/common"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

func (h *Controller) GetStatistics(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	logger.Info("Retrieved statistics request successfully")

	var statistics map[string]int
	if statistics, err = calculateStatistics(PlatformZhihu, "canglimo", h.zhihuDBService); err != nil {
		logger.Error("Failed to calculate statistics", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, ErrResponse{Message: "Failed to calculate statistics"})
	}

	return c.JSON(http.StatusOK, statistics)
}

func calculateStatistics(_, author string, d zhihuDB.DB) (map[string]int, error) {
	answers, err := d.GetAnswerAfter(author, getLastYearDate())
	if err != nil {
		return nil, fmt.Errorf("failed to get answers: %w", err)
	}

	return lo.CountValuesBy(answers, func(a zhihuDB.Answer) string { return a.CreateAt.Format("2006-01-02") }), nil
}

func getLastYearDate() time.Time {
	return time.Date(time.Now().Year()-1, time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, config.C.BJT)
}
