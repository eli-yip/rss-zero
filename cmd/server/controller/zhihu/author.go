package controller

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

func (h *Controller) AuthorName(c echo.Context) (err error) {
	type (
		Response struct {
			ID       string `json:"id"`
			Nickname string `json:"nickname"`
		}

		ErrResponse struct {
			Message string `json:"message"`
		}
	)

	logger := c.Get("logger").(*zap.Logger)

	id := c.Param("id")

	nickname, err := h.db.GetAuthorName(id)
	if err != nil {
		logger.Error("Failed to get author name", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, &ErrResponse{Message: "failed to get author name"})
	}
	logger.Info("Get author name successfully", zap.String("author_id", id), zap.String("nickname", nickname))

	return c.JSON(http.StatusOK, &Response{
		ID:       id,
		Nickname: nickname,
	})
}
