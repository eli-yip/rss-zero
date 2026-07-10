package tkblog

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/eli-yip/rss-zero/pkg/httputil"
	tkblog "github.com/eli-yip/rss-zero/pkg/routers/tkblog"
)

// Crawl launches a background full crawl of one tkblog category (xfocus or baidu)
// and returns a job_id for log correlation. The :category path param is validated
// against the allowed set (trust boundary) and rejected with 400 if unknown; it
// rejects with 409 if a crawl is already running for that category (one per
// category, per process); failures are logged and pushed to Bark under the job_id.
func (h *Controller) Crawl(c echo.Context) error {
	category := c.Param("category")
	if !tkblog.ValidCategory(category) {
		return httputil.NewHTTPError(http.StatusBadRequest, "category must be xfocus or baidu")
	}

	jobID, err := tkblog.StartCrawl(h.db, h.notifier, category, h.logger)
	if err != nil {
		if errors.Is(err, tkblog.ErrCrawlRunning) {
			return httputil.NewHTTPError(http.StatusConflict, err.Error())
		}
		return httputil.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusAccepted, httputil.NewResp("tkblog crawl started",
		map[string]string{"job_id": jobID}))
}
