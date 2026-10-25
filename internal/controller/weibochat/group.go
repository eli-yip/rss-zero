package weibochat

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/labstack/echo/v5"
)

func (h *Controller) ListGroups(c *echo.Context) error {
	owner, err := ownerUsername(c)
	if err != nil {
		return err
	}
	items, err := h.groupResponses(c.Request().Context(), owner)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httputil.NewResp("weibo groups", GroupListResponse{Items: items}))
}

func (h *Controller) groupResponses(ctx context.Context, owner string) ([]GroupResponse, error) {
	groups, err := h.store.ListGroups(ctx)
	if err != nil {
		return nil, err
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
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.GroupID)
	}
	names, err := h.store.GetGroupNames(ctx, ids)
	if err != nil {
		return nil, err
	}
	unread, err := h.store.GroupsWithUnread(ctx, owner, ids)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].HasUnread = unread[items[i].GroupID]
		items[i].Name = names[items[i].GroupID]
	}
	return items, nil
}

type UpdateGroupRequest struct {
	Name *string `json:"name"`
}

func (h *Controller) UpdateGroup(c *echo.Context) error {
	id := c.Param("group_id")
	if !weibochat.MessageID(id).Valid() {
		return httputil.NewHTTPError(http.StatusBadRequest, "invalid group_id")
	}
	var request UpdateGroupRequest
	if err := c.Bind(&request); err != nil || request.Name == nil {
		return httputil.NewHTTPError(http.StatusBadRequest, "name is required")
	}
	name := strings.TrimSpace(*request.Name)
	if utf8.RuneCountInString(name) > 100 {
		return httputil.NewHTTPError(http.StatusBadRequest, "name must not exceed 100 characters")
	}
	owner, err := ownerUsername(c)
	if err != nil {
		return err
	}
	groups, err := h.groupResponses(c.Request().Context(), owner)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if group.GroupID != id {
			continue
		}
		if err := h.store.SetGroupName(c.Request().Context(), id, name); err != nil {
			return err
		}
		group.Name = name
		return c.JSON(http.StatusOK, httputil.NewResp("weibo group updated", group))
	}
	return httputil.NewHTTPError(http.StatusNotFound, "group not found")
}
