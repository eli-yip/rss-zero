package weibochat

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/labstack/echo/v5"
)

type UserPreferenceRequest struct {
	IsStarred *bool `json:"is_starred"`
	IsBlocked *bool `json:"is_blocked"`
}

type UserPreferenceListResponse struct {
	Items        []SenderResponse `json:"items"`
	NextAfterUID *string          `json:"next_after_uid"`
}

func (h *Controller) UpdateUserPreference(c *echo.Context) error {
	uid := c.Param("uid")
	if !weibochat.MessageID(uid).Valid() {
		return httputil.NewHTTPError(http.StatusBadRequest, "invalid uid")
	}
	owner, err := ownerUsername(c)
	if err != nil {
		return err
	}
	var request UserPreferenceRequest
	if err = c.Bind(&request); err != nil {
		return httputil.NewHTTPError(http.StatusBadRequest, "invalid preference body")
	}
	if request.IsStarred == nil && request.IsBlocked == nil {
		return httputil.NewHTTPError(http.StatusBadRequest, "at least one preference is required")
	}
	pref, err := h.store.UpdateUserPreference(c.Request().Context(), owner, uid, weibochat.UserPreferencePatch{IsStarred: request.IsStarred, IsBlocked: request.IsBlocked})
	if err != nil {
		return err
	}
	user, err := h.store.GetUser(c.Request().Context(), uid)
	if err != nil {
		return err
	}
	if user == nil {
		user = &weibochat.User{UID: uid}
	}
	result := senderResponse(*user)
	result.IsStarred = pref.IsStarred
	result.IsBlocked = pref.IsBlocked
	return c.JSON(http.StatusOK, httputil.NewResp("weibo user preference", result))
}

func (h *Controller) ListUserPreferences(c *echo.Context) error {
	owner, err := ownerUsername(c)
	if err != nil {
		return err
	}
	query := c.QueryParams()
	starred, err := optionalBool(query, "starred")
	if err != nil {
		return err
	}
	blocked, err := optionalBool(query, "blocked")
	if err != nil {
		return err
	}
	filter := weibochat.UserPreferenceFilter{Starred: starred, Blocked: blocked, Limit: 50}
	if values, ok := query["after_uid"]; ok {
		if len(values) != 1 || !weibochat.MessageID(values[0]).Valid() {
			return httputil.NewHTTPError(http.StatusBadRequest, "invalid after_uid")
		}
		filter.AfterUID = values[0]
	}
	if values, ok := query["limit"]; ok {
		if len(values) != 1 {
			return httputil.NewHTTPError(http.StatusBadRequest, "limit requires one value")
		}
		filter.Limit, err = strconv.Atoi(values[0])
		if err != nil || filter.Limit < 1 || filter.Limit > 100 {
			return httputil.NewHTTPError(http.StatusBadRequest, "limit must be between 1 and 100")
		}
	}
	page, err := h.store.ListUserPreferences(c.Request().Context(), owner, filter)
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(page.Items))
	for _, p := range page.Items {
		ids = append(ids, p.UID)
	}
	users, err := h.store.GetUsers(c.Request().Context(), ids)
	if err != nil {
		return err
	}
	byUID := make(map[string]weibochat.User, len(users))
	for _, user := range users {
		byUID[user.UID] = user
	}
	response := UserPreferenceListResponse{Items: make([]SenderResponse, 0, len(page.Items))}
	for _, p := range page.Items {
		user, ok := byUID[p.UID]
		if !ok {
			user = weibochat.User{UID: p.UID}
		}
		item := senderResponse(user)
		item.IsStarred = p.IsStarred
		item.IsBlocked = p.IsBlocked
		response.Items = append(response.Items, item)
	}
	if page.HasMore {
		uid := page.Items[len(page.Items)-1].UID
		response.NextAfterUID = &uid
	}
	return c.JSON(http.StatusOK, httputil.NewResp("weibo user preferences", response))
}

func optionalBool(query url.Values, name string) (*bool, error) {
	values, ok := query[name]
	if !ok {
		return nil, nil
	}
	if len(values) != 1 || (values[0] != "true" && values[0] != "false") {
		return nil, httputil.NewHTTPError(http.StatusBadRequest, name+" must be true or false")
	}
	value := values[0] == "true"
	return &value, nil
}
