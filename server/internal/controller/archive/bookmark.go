package archive

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/eli-yip/rss-zero/internal/controller/common"
	utils "github.com/eli-yip/rss-zero/internal/utils"
	bookmarkDB "github.com/eli-yip/rss-zero/pkg/bookmark/db"

	"github.com/labstack/echo/v4"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

func (h *Controller) GetBookmarkList(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	var req BookmarkRequest
	if err = c.Bind(&req); err != nil {
		logger.Error("failed to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, common.WrapResp(err.Error()))
	}
	logger.Info("bind request successfully")

	username := c.Get("username").(string)
	var startDate, endDate time.Time
	startDate, err = utils.ParseStartTime(req.StartDate)
	if err != nil {
		logger.Error("Failed to parse start date", zap.Error(err), zap.String("start_date", req.StartDate))
		return c.JSON(http.StatusBadRequest, ErrResponse{Message: "Invalid start date"})
	}
	endDate, err = utils.ParseEndTime(req.EndDate)
	if err != nil {
		logger.Error("Failed to parse end date", zap.Error(err), zap.String("end_date", req.EndDate))
		return c.JSON(http.StatusBadRequest, ErrResponse{Message: "Invalid end date"})
	}
	queryParam := &bookmarkDB.BookmarkQuery{
		StartTime: startDate,
		EndTime:   endDate,
		TimeBy:    bookmarkDB.BookmarkQueryTime(req.DateBy),
		Page:      req.Page,
		Order:     req.DateBy,
		Orderby:   bookmarkDB.BookmarkQueryTime(req.OrderBy),
	}
	if req.Tags != nil {
		if req.Tags.NoTag {
			queryParam.Tag = &bookmarkDB.TagFilter{NoTag: true}
		} else {
			// TODO: use set to handle duplicate tags
			queryParam.Tag = &bookmarkDB.TagFilter{
				Include: req.Tags.Include,
				Exclude: req.Tags.Exclude,
			}
		}
	}
	bookmarks, err := h.bookmarkDBService.GetBookmarkByUser(username, queryParam)
	if err != nil {
		logger.Error("failed to get bookmarks", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}

	answerBookmarks := lo.Filter(bookmarks, func(b bookmarkDB.Bookmark, _ int) bool { return b.ContentType == bookmarkDB.ContentTypeAnswer })
	answerIDs := lo.Map(answerBookmarks, func(b bookmarkDB.Bookmark, _ int) string { return b.ContentID })
	answerTags, err := h.bookmarkDBService.GetTagsFromAnswer(answerIDs)
	if err != nil {
		logger.Error("failed to get tags from answers", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}
	err = nil
	answerIDsInt := lo.Map(answerIDs, func(id string, _ int) int {
		var idInt int
		idInt, err = strconv.Atoi(id)
		if err != nil {
			logger.Error("failed to convert answer ID to int", zap.Error(err), zap.String("answer_id", id))
			err = fmt.Errorf("failed to convert answer ID to int: %w", err)
			return 0
		}
		return idInt
	})
	if err != nil {
		logger.Error("failed to convert answer ID to int", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}
	answers, err := h.zhihuDBService.FetchAnswerByIDs(answerIDsInt)
	if err != nil {
		logger.Error("failed to fetch answers by IDs", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}
	answerIDToBookmarkID := lo.SliceToMap(answerBookmarks, func(b bookmarkDB.Bookmark) (int, string) {
		var answerID int
		answerID, err = strconv.Atoi(b.ContentID)
		if err != nil {
			err = fmt.Errorf("failed to convert content ID %s to int: %w", b.ContentID, err)
			return 0, ""
		}
		return answerID, b.ID
	})
	if err != nil {
		logger.Error("failed to convert bookmark ID to int", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}
	answerTopics, err := buildTopicMapFromAnswer(answers, answerIDToBookmarkID, answerTags, h.zhihuDBService)
	if err != nil {
		logger.Error("failed to build topic map from answers", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}

	pinBookmarks := lo.Filter(bookmarks, func(b bookmarkDB.Bookmark, _ int) bool { return b.ContentType == bookmarkDB.ContentTypePin })
	pinIDs := lo.Map(pinBookmarks, func(b bookmarkDB.Bookmark, _ int) string { return b.ContentID })
	pinTags, err := h.bookmarkDBService.GetTagsFromPin(pinIDs)
	if err != nil {
		logger.Error("failed to get tags from pins", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}
	pinIDsInt := lo.Map(pinIDs, func(id string, _ int) int {
		var idInt int
		idInt, err = strconv.Atoi(id)
		if err != nil {
			err = fmt.Errorf("failed to convert pin ID %s to int: %w", id, err)
			return 0
		}
		return idInt
	})
	if err != nil {
		logger.Error("failed to convert pin ID to int", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}
	pins, err := h.zhihuDBService.FetchPinByIDs(pinIDsInt)
	if err != nil {
		logger.Error("failed to fetch pins by IDs", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}
	pinIDToBookmarkIDMap := lo.SliceToMap(pinBookmarks, func(b bookmarkDB.Bookmark) (int, string) {
		var pinID int
		pinID, err = strconv.Atoi(b.ContentID)
		if err != nil {
			err = fmt.Errorf("failed to convert content ID %s to int: %w", b.ContentID, err)
			return 0, ""
		}
		return pinID, b.ID
	})
	if err != nil {
		logger.Error("failed to convert bookmark ID to int", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}
	pinTopics, err := buildTopicMapFromPin(pins, pinIDToBookmarkIDMap, pinTags, h.zhihuDBService)
	if err != nil {
		logger.Error("failed to build topic map from pins", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}

	count, err := h.bookmarkDBService.CountBookmarkByUser(username, queryParam)
	if err != nil {
		logger.Error("failed to count bookmarks", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}

	const pageSize = 20
	totalPage := int(math.Ceil(float64(count) / float64(pageSize)))
	response := &ArchiveResponse{
		Count: count,
		Paging: Paging{
			Total:   totalPage,
			Current: req.Page,
		},
		ResponseBase: ResponseBase{Topics: make([]Topic, 0, len(bookmarks))},
	}

	for b := range slices.Values(bookmarks) {
		switch b.ContentType {
		case bookmarkDB.ContentTypeAnswer:
			response.Topics = append(response.Topics, answerTopics[b.ContentID])
		case bookmarkDB.ContentTypePin:
			response.Topics = append(response.Topics, pinTopics[b.ContentID])
		}
	}

	logger.Info("Get bookmark list successfully", zap.String("username", username), zap.Int("page", req.Page), zap.Int("total_page", totalPage))

	return c.JSON(http.StatusOK, common.WrapRespWithData("success", response))
}

// PutBookmark handles the request to create a new bookmark
func (h *Controller) PutBookmark(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)
	user := c.Get("username").(string)

	var req NewBookmarkRequest
	if err = c.Bind(&req); err != nil {
		logger.Error("failed to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, common.WrapResp(err.Error()))
	}
	logger.Info("bind request successfully")

	_, err = h.bookmarkDBService.GetBookmarkByContent(user, req.ContentType, req.ContentID)
	if err == nil {
		logger.Error("bookmark already exists", zap.String("content_id", req.ContentID))
		return c.JSON(http.StatusBadRequest, common.WrapResp("bookmark already exists"))
	} else if !errors.Is(err, bookmarkDB.ErrNoBookmark) {
		logger.Error("failed to get bookmark", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}

	var b *bookmarkDB.Bookmark
	switch req.ContentType {
	case bookmarkDB.ContentTypeAnswer:
		answerIDInt, err := strconv.Atoi(req.ContentID)
		if err != nil {
			logger.Error("failed to convert answer ID to int", zap.Error(err))
			return c.JSON(http.StatusBadRequest, common.WrapResp(err.Error()))
		}
		_, err = h.zhihuDBService.GetAnswer(answerIDInt)
		if err != nil {
			logger.Error("failed to get answer", zap.Error(err))
			return c.JSON(http.StatusBadRequest, common.WrapResp(err.Error()))
		}
		b, err = h.bookmarkDBService.NewBookmark(user, bookmarkDB.ContentTypeAnswer, req.ContentID)
		if err != nil {
			logger.Error("failed to create bookmark", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
		}
	case bookmarkDB.ContentTypePin:
		pinIDInt, err := strconv.Atoi(req.ContentID)
		if err != nil {
			logger.Error("failed to convert pin ID to int", zap.Error(err))
			return c.JSON(http.StatusBadRequest, common.WrapResp(err.Error()))
		}
		_, err = h.zhihuDBService.GetPin(pinIDInt)
		if err != nil {
			logger.Error("failed to get pin", zap.Error(err))
			return c.JSON(http.StatusBadRequest, common.WrapResp(err.Error()))
		}
		b, err = h.bookmarkDBService.NewBookmark(user, bookmarkDB.ContentTypePin, req.ContentID)
		if err != nil {
			logger.Error("failed to create bookmark", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
		}
	default:
		logger.Error("invalid content type", zap.Int("content_type", req.ContentType))
		return c.JSON(http.StatusBadRequest, common.WrapResp("invalid content type"))
	}

	logger.Info("Create bookmark successfully", zap.String("bookmark_id", b.ID))

	return c.JSON(http.StatusOK, common.WrapRespWithData("success", &NewBookmarkResponse{BookmarkID: b.ID}))
}

func (h *Controller) PatchBookmark(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)
	user := c.Get("username").(string)
	bookmarkID := c.Param("id")
	if bookmarkID == "" {
		logger.Error("bookmark ID is empty")
		return c.JSON(http.StatusBadRequest, common.WrapResp("bookmark ID is empty"))
	}
	var req PutBookmarkRequest
	if err = c.Bind(&req); err != nil {
		logger.Error("failed to bind request", zap.Error(err))
		return c.JSON(http.StatusBadRequest, common.WrapResp(err.Error()))
	}
	logger.Info("bind request successfully")

	b, err := h.bookmarkDBService.GetBookmark(user, bookmarkID)
	if err != nil {
		if errors.Is(err, bookmarkDB.ErrNoBookmark) {
			logger.Error("bookmark not found", zap.Error(err))
			return c.JSON(http.StatusNotFound, common.WrapResp("bookmark not found"))
		}
		logger.Error("failed to get bookmark", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}

	if req.Comment == nil && req.Note == nil && req.Tags == nil {
		logger.Error("no fields to update")
		return c.JSON(http.StatusBadRequest, common.WrapResp("no fields to update"))
	}

	comment, note := b.Comment, b.Note
	if req.Comment != nil {
		comment = *req.Comment
	}
	if req.Note != nil {
		note = *req.Note
	}
	_, err = h.bookmarkDBService.UpdateBookmark(bookmarkID, comment, note)
	if err != nil {
		logger.Error("failed to update bookmark", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}

	if req.Tags != nil {
		err = h.bookmarkDBService.UpdateTag(bookmarkID, req.Tags)
		if err != nil {
			logger.Error("failed to update tags", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
		}
	}

	logger.Info("Update bookmark successfully", zap.String("bookmark_id", bookmarkID))

	return c.JSON(http.StatusOK, common.WrapResp("success"))
}

func (h *Controller) DeleteBookmark(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)
	user := c.Get("username").(string)
	bookmarkID := c.Param("id")

	if bookmarkID == "" {
		logger.Error("bookmark ID is empty")
		return c.JSON(http.StatusBadRequest, common.WrapResp("bookmark ID is empty"))
	}

	_, err = h.bookmarkDBService.GetBookmark(user, bookmarkID)
	if err != nil {
		if errors.Is(err, bookmarkDB.ErrNoBookmark) {
			logger.Error("bookmark not found", zap.Error(err))
			return c.JSON(http.StatusNotFound, common.WrapResp("bookmark not found"))
		}
		logger.Error("failed to get bookmark", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}

	if err = h.bookmarkDBService.RemoveBookmark(bookmarkID); err != nil {
		logger.Error("failed to remove bookmark", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}

	logger.Info("Delete bookmark successfully", zap.String("bookmark_id", bookmarkID))

	return c.JSON(http.StatusOK, common.WrapResp("success"))
}

func (h *Controller) GetAllTags(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)
	user := c.Get("username").(string)

	tagCounts, err := h.bookmarkDBService.GetTagCountByUser(user)
	if err != nil {
		logger.Error("failed to get tag counts", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, common.WrapResp(err.Error()))
	}
	logger.Info("Get tag counts successfully", zap.String("username", user))

	response := struct {
		Tags []bookmarkDB.TagCount `json:"tags"`
	}{
		Tags: tagCounts,
	}

	logger.Info("Get all tags successfully", zap.String("username", user))

	return c.JSON(http.StatusOK, common.WrapRespWithData("success", response))
}
