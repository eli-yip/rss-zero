package weibochat

import (
	"errors"
	"net/http"
	"slices"

	"github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/labstack/echo/v5"
)

type ReadStateRequest struct {
	Action    string  `json:"action"`
	MessageID *string `json:"message_id"`
	Revision  *int64  `json:"revision"`
}

func (h *Controller) readStateScope(c *echo.Context) (owner, groupID string, err error) {
	groupID = c.Param("group_id")
	if !weibochat.MessageID(groupID).Valid() {
		return "", "", httputil.NewHTTPError(http.StatusBadRequest, "invalid group_id")
	}
	owner, err = ownerUsername(c)
	if err != nil {
		return
	}
	if slices.Contains(h.groupIDs, groupID) {
		return
	}
	var exists bool
	exists, err = h.store.HasGroup(c.Request().Context(), groupID)
	if err == nil && !exists {
		err = httputil.NewHTTPError(http.StatusNotFound, "group not found")
	}
	return
}

func (h *Controller) GetReadState(c *echo.Context) error {
	owner, groupID, err := h.readStateScope(c)
	if err != nil {
		return err
	}
	result, err := h.store.GetReadState(c.Request().Context(), owner, groupID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httputil.NewResp("weibo read state", result))
}

func (h *Controller) UpdateReadState(c *echo.Context) error {
	owner, groupID, err := h.readStateScope(c)
	if err != nil {
		return err
	}
	var request ReadStateRequest
	if err = c.Bind(&request); err != nil || request.Revision == nil {
		return httputil.NewHTTPError(http.StatusBadRequest, "revision and valid read state body are required")
	}
	result, err := h.store.UpdateReadState(c.Request().Context(), owner, groupID, weibochat.ReadStatePatch{Action: request.Action, MessageID: request.MessageID, Revision: *request.Revision})
	switch {
	case errors.Is(err, weibochat.ErrReadStateConflict):
		return httputil.NewHTTPError(http.StatusConflict, err.Error())
	case errors.Is(err, weibochat.ErrInvalidReadState):
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	case errors.Is(err, weibochat.ErrMessageNotFound):
		return httputil.NewHTTPError(http.StatusNotFound, err.Error())
	case err != nil:
		return err
	}
	return c.JSON(http.StatusOK, httputil.NewResp("weibo read state updated", result))
}
