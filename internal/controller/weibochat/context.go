package weibochat

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/labstack/echo/v5"
)

func (h *Controller) GetMessageContext(c *echo.Context) error {
	groupID, messageID := c.Param("group_id"), weibochat.MessageID(c.Param("message_id"))
	if !weibochat.MessageID(groupID).Valid() || !messageID.Valid() {
		return httputil.NewHTTPError(http.StatusBadRequest, "invalid group_id or message_id")
	}
	query := c.QueryParams()
	for _, name := range []string{"q", "sender_id", "sent_from", "sent_before", "asset_kind", "before_message_id", "after_message_id", "limit"} {
		if query.Has(name) {
			return httputil.NewHTTPError(http.StatusBadRequest, name+" is not supported by message context")
		}
	}
	before, err := parseContextLimit(query, "before_limit")
	if err != nil {
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	after, err := parseContextLimit(query, "after_limit")
	if err != nil {
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	highlight, err := parseHighlightQuery(query, weibochat.SearchQuery{})
	if err != nil {
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	ctx := c.Request().Context()
	result, err := h.store.GetMessageContext(ctx, groupID, messageID, before, after)
	if errors.Is(err, weibochat.ErrMessageNotFound) {
		return httputil.NewHTTPError(http.StatusNotFound, "message not found")
	}
	if err != nil {
		return err
	}
	items, err := h.messageResponses(ctx, groupID, result.Messages, highlight)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httputil.NewResp("weibo message context", MessageContextResponse{
		AnchorMessageID: string(result.AnchorMessageID), Items: items,
		HasMoreBefore: result.HasMoreBefore, HasMoreAfter: result.HasMoreAfter,
	}))
}

func parseContextLimit(query url.Values, name string) (int, error) {
	values, present := query[name]
	if !present {
		return 20, nil
	}
	if len(values) != 1 {
		return 0, fmt.Errorf("%s requires one value", name)
	}
	value, err := strconv.Atoi(values[0])
	if err != nil || value < 0 || value > 50 {
		return 0, fmt.Errorf("%s must be between 0 and 50", name)
	}
	return value, nil
}
