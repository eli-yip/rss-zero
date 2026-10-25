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
	owner, err := ownerUsername(c)
	if err != nil {
		return err
	}
	users, err := h.store.ListSenders(c.Request().Context(), groupID)
	if err != nil {
		return err
	}
	uids := make([]string, 0, len(users))
	for _, user := range users {
		uids = append(uids, user.UID)
	}
	preferences, err := h.store.GetUserPreferences(c.Request().Context(), owner, uids)
	if err != nil {
		return err
	}
	items := make([]SenderResponse, 0, len(users))
	for _, user := range users {
		item := senderResponse(user)
		item.IsStarred = preferences[user.UID].IsStarred
		item.IsBlocked = preferences[user.UID].IsBlocked
		items = append(items, item)
	}
	return c.JSON(http.StatusOK, httputil.NewResp("weibo senders", SenderListResponse{Items: items}))
}
