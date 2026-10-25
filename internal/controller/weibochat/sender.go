package weibochat

import (
	"net/http"

	"github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/labstack/echo/v5"
)

func (h *Controller) ListSenders(c *echo.Context) error {
	groupID := c.Param("group_id")
	if !weibochat.MessageID(groupID).Valid() {
		return httputil.NewHTTPError(http.StatusBadRequest, "invalid group_id")
	}
	users, err := h.store.ListSenders(c.Request().Context(), groupID)
	if err != nil {
		return err
	}
	items := make([]SenderResponse, 0, len(users))
	for _, user := range users {
		items = append(items, senderResponse(user))
	}
	return c.JSON(http.StatusOK, httputil.NewResp("weibo senders", SenderListResponse{Items: items}))
}
