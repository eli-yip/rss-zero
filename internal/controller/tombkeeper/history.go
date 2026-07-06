package tombkeeper

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	tk "github.com/eli-yip/rss-zero/pkg/routers/tombkeeper"
)

// History launches a background backfill of tombkeeper posts within
// [start_date, end_date] (YYYY-MM-DD, Asia/Shanghai) and returns a job_id for log
// correlation. It rejects with 409 if a backfill is already running (one per
// process); the crawl runs newest→oldest until the window is exhausted, and a
// failure is logged and pushed to Bark under the same job_id.
func (h *Controller) History(c echo.Context) error {
	var req struct {
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	if err := c.Bind(&req); err != nil {
		return httputil.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	start, ok1 := parseDate(req.StartDate)
	end, ok2 := parseDate(req.EndDate)
	if !ok1 || !ok2 {
		return httputil.NewHTTPError(http.StatusBadRequest, "start_date and end_date must be YYYY-MM-DD")
	}
	if start.After(end) {
		return httputil.NewHTTPError(http.StatusBadRequest, "start_date must not be after end_date")
	}

	jobID, err := tk.StartHistory(h.db, h.file, h.notifier, req.StartDate, req.EndDate, h.logger)
	if err != nil {
		if errors.Is(err, tk.ErrHistoryRunning) {
			return httputil.NewHTTPError(http.StatusConflict, err.Error())
		}
		return httputil.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusAccepted, httputil.NewResp("tombkeeper history crawl started",
		map[string]string{"job_id": jobID}))
}

func parseDate(s string) (time.Time, bool) {
	t, err := time.ParseInLocation("2006-01-02", s, config.C.BJT)
	return t, err == nil
}
