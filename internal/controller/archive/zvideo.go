package archive

import (
	"fmt"
	"net/http"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

func (h *Controller) ZvideoList(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	logger.Info("Retrieve zvideo list request")

	zvideos, err := h.zhihuDBService.GetZvideos()
	if err != nil {
		logger.Error("Failed to get zvideo list from db", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, ErrResponse{Message: "Failed to get zvideo list from db"})
	}

	resp := ZvideoResponse{
		Zvideos: lo.Map(zvideos, func(z db.Zvideo, _ int) Zvideo {
			return Zvideo{
				ID:          z.ID,
				Url:         fmt.Sprintf("https://www.zhihu.com/zvideo/%s", z.ID),
				Title:       z.Filename[6:],
				PublishedAt: z.PublishedAt.Format("2006-01-02"),
			}
		}),
	}

	return c.JSON(http.StatusOK, resp)
}
