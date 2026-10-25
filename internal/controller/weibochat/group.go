package weibochat

import (
	"net/http"
	"slices"

	"github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/labstack/echo/v5"
)

func (h *Controller) ListGroups(c *echo.Context) error {
	groups, err := h.store.ListGroups(c.Request().Context())
	if err != nil {
		return err
	}
	byID := make(map[string]GroupResponse, len(groups)+len(h.groupIDs))
	for _, group := range groups {
		byID[group.GroupID] = GroupResponse{GroupID: group.GroupID, InitializedAt: utc(group.InitializedAt), LastSyncedAt: utc(group.LastSyncedAt)}
	}
	for _, id := range h.groupIDs {
		group := byID[id]
		group.GroupID, group.Configured = id, true
		byID[id] = group
	}
	items := make([]GroupResponse, 0, len(byID))
	for _, group := range byID {
		items = append(items, group)
	}
	slices.SortFunc(items, func(a, b GroupResponse) int {
		return weibochat.MessageID(a.GroupID).Compare(weibochat.MessageID(b.GroupID))
	})
	return c.JSON(http.StatusOK, httputil.NewResp("weibo groups", GroupListResponse{Items: items}))
}
