package weibochat

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

func bookmarkBlockResponse(b weibochat.BookmarkBlock) BookmarkBlockResponse {
	return BookmarkBlockResponse{b.ID, b.Title, b.Tags, b.MessageCount, b.CreatedAt.UTC(), b.UpdatedAt.UTC()}
}
func optionalCursor(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
func bookmarkError(err error) error {
	if errors.Is(err, weibochat.ErrBookmarkNotFound) || errors.Is(err, weibochat.ErrMessageNotFound) {
		return httputil.NewHTTPError(http.StatusNotFound, err.Error())
	}
	if errors.Is(err, weibochat.ErrInvalidBookmarkCursor) {
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	return err
}
func bookmarkID(c *echo.Context) (string, error) {
	id, err := uuid.Parse(c.Param("block_id"))
	if err != nil {
		return "", httputil.NewHTTPError(http.StatusBadRequest, "invalid block_id")
	}
	return id.String(), nil
}
func normalizeBookmarkTags(tags []string) ([]string, error) {
	if len(tags) > 20 {
		return nil, fmt.Errorf("at most 20 tags are allowed")
	}
	result := []string{}
	seen := map[string]bool{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" || utf8.RuneCountInString(tag) > 50 {
			return nil, fmt.Errorf("tags must contain 1 to 50 characters")
		}
		if !seen[tag] {
			result = append(result, tag)
			seen[tag] = true
		}
	}
	return result, nil
}
func validateBookmarkRefs(refs []weibochat.MessageRef) error {
	if len(refs) < 1 || len(refs) > 100 {
		return fmt.Errorf("messages must contain 1 to 100 items")
	}
	for _, ref := range refs {
		if !weibochat.MessageID(ref.GroupID).Valid() || !weibochat.MessageID(ref.MessageID).Valid() {
			return fmt.Errorf("invalid group_id or message_id")
		}
	}
	return nil
}
func bookmarkPaging(query url.Values) (string, int, error) {
	cursor := ""
	limit := 50
	for _, name := range []string{"cursor", "limit"} {
		v, ok := query[name]
		if !ok {
			continue
		}
		if len(v) != 1 || v[0] == "" {
			return "", 0, fmt.Errorf("%s requires one non-empty value", name)
		}
		if name == "cursor" {
			cursor = v[0]
		} else {
			n, err := strconv.Atoi(v[0])
			if err != nil || n < 1 || n > 100 {
				return "", 0, fmt.Errorf("limit must be between 1 and 100")
			}
			limit = n
		}
	}
	return cursor, limit, nil
}
func (h *Controller) CreateBookmarkBlock(c *echo.Context) error {
	owner, err := ownerUsername(c)
	if err != nil {
		return err
	}
	var req CreateBookmarkBlockRequest
	if err = c.Bind(&req); err != nil {
		return httputil.NewHTTPError(400, "invalid request body")
	}
	req.Title = strings.TrimSpace(req.Title)
	if utf8.RuneCountInString(req.Title) > 200 {
		return httputil.NewHTTPError(400, "title must not exceed 200 characters")
	}
	tags, err := normalizeBookmarkTags(req.Tags)
	if err != nil {
		return httputil.NewHTTPError(400, err.Error())
	}
	if err = validateBookmarkRefs(req.Messages); err != nil {
		return httputil.NewHTTPError(400, err.Error())
	}
	block, err := h.store.CreateBookmarkBlock(c.Request().Context(), owner, req.Title, tags, req.Messages)
	if err != nil {
		return bookmarkError(err)
	}
	return c.JSON(http.StatusCreated, httputil.NewResp("bookmark block created", bookmarkBlockResponse(block)))
}
func (h *Controller) GetBookmarkBlock(c *echo.Context) error {
	owner, err := ownerUsername(c)
	if err != nil {
		return err
	}
	id, err := bookmarkID(c)
	if err != nil {
		return err
	}
	block, err := h.store.GetBookmarkBlock(c.Request().Context(), owner, id)
	if err != nil {
		return bookmarkError(err)
	}
	return c.JSON(200, httputil.NewResp("bookmark block", bookmarkBlockResponse(block)))
}
func (h *Controller) UpdateBookmarkBlock(c *echo.Context) error {
	owner, err := ownerUsername(c)
	if err != nil {
		return err
	}
	id, err := bookmarkID(c)
	if err != nil {
		return err
	}
	var req UpdateBookmarkBlockRequest
	if err = c.Bind(&req); err != nil {
		return httputil.NewHTTPError(400, "invalid request body")
	}
	if req.Title == nil && req.Tags == nil {
		return httputil.NewHTTPError(400, "title or tags is required")
	}
	if req.Title != nil {
		*req.Title = strings.TrimSpace(*req.Title)
		if utf8.RuneCountInString(*req.Title) > 200 {
			return httputil.NewHTTPError(400, "title must not exceed 200 characters")
		}
	}
	if req.Tags != nil {
		tags, e := normalizeBookmarkTags(*req.Tags)
		if e != nil {
			return httputil.NewHTTPError(400, e.Error())
		}
		req.Tags = &tags
	}
	block, err := h.store.UpdateBookmarkBlock(c.Request().Context(), owner, id, req.Title, req.Tags)
	if err != nil {
		return bookmarkError(err)
	}
	return c.JSON(200, httputil.NewResp("bookmark block updated", bookmarkBlockResponse(block)))
}
func (h *Controller) DeleteBookmarkBlock(c *echo.Context) error {
	owner, err := ownerUsername(c)
	if err != nil {
		return err
	}
	id, err := bookmarkID(c)
	if err != nil {
		return err
	}
	if err = h.store.DeleteBookmarkBlock(c.Request().Context(), owner, id); err != nil {
		return bookmarkError(err)
	}
	return c.NoContent(http.StatusNoContent)
}
func (h *Controller) AddBookmarkMessages(c *echo.Context) error {
	owner, err := ownerUsername(c)
	if err != nil {
		return err
	}
	id, err := bookmarkID(c)
	if err != nil {
		return err
	}
	var req AddBookmarkMessagesRequest
	if err = c.Bind(&req); err != nil {
		return httputil.NewHTTPError(400, "invalid request body")
	}
	if err = validateBookmarkRefs(req.Messages); err != nil {
		return httputil.NewHTTPError(400, err.Error())
	}
	if err = h.store.AddBookmarkMessages(c.Request().Context(), owner, id, req.Messages); err != nil {
		return bookmarkError(err)
	}
	return c.NoContent(http.StatusNoContent)
}
func (h *Controller) RemoveBookmarkMessage(c *echo.Context) error {
	owner, err := ownerUsername(c)
	if err != nil {
		return err
	}
	id, err := bookmarkID(c)
	if err != nil {
		return err
	}
	ref := weibochat.MessageRef{GroupID: c.Param("group_id"), MessageID: c.Param("message_id")}
	if err = validateBookmarkRefs([]weibochat.MessageRef{ref}); err != nil {
		return httputil.NewHTTPError(400, err.Error())
	}
	if err = h.store.RemoveBookmarkMessage(c.Request().Context(), owner, id, ref); err != nil {
		return bookmarkError(err)
	}
	return c.NoContent(http.StatusNoContent)
}
func (h *Controller) ListBookmarkBlocks(c *echo.Context) error { return h.listBookmarkBlocks(c, nil) }
func (h *Controller) ListMessageBookmarkBlocks(c *echo.Context) error {
	ref := weibochat.MessageRef{GroupID: c.Param("group_id"), MessageID: c.Param("message_id")}
	if err := validateBookmarkRefs([]weibochat.MessageRef{ref}); err != nil {
		return httputil.NewHTTPError(400, err.Error())
	}
	return h.listBookmarkBlocks(c, &ref)
}
func (h *Controller) listBookmarkBlocks(c *echo.Context, ref *weibochat.MessageRef) error {
	owner, err := ownerUsername(c)
	if err != nil {
		return err
	}
	q := c.QueryParams()
	cursor, limit, err := bookmarkPaging(q)
	if err != nil {
		return httputil.NewHTTPError(400, err.Error())
	}
	title := ""
	if v, ok := q["title"]; ok {
		if len(v) != 1 || utf8.RuneCountInString(v[0]) > 200 {
			return httputil.NewHTTPError(400, "invalid title")
		}
		title = strings.TrimSpace(v[0])
	}
	tags, err := normalizeBookmarkTags(q["tag"])
	if err != nil {
		return httputil.NewHTTPError(400, err.Error())
	}
	page, err := h.store.ListBookmarkBlocks(c.Request().Context(), owner, weibochat.BookmarkBlockFilter{Title: title, Tags: tags, Cursor: cursor, Limit: limit, Message: ref})
	if err != nil {
		return bookmarkError(err)
	}
	items := make([]BookmarkBlockResponse, 0, len(page.Blocks))
	for _, block := range page.Blocks {
		items = append(items, bookmarkBlockResponse(block))
	}
	return c.JSON(200, httputil.NewResp("bookmark blocks", BookmarkBlockListResponse{items, optionalCursor(page.NextCursor)}))
}
func (h *Controller) ListBookmarkMessages(c *echo.Context) error {
	owner, err := ownerUsername(c)
	if err != nil {
		return err
	}
	id, err := bookmarkID(c)
	if err != nil {
		return err
	}
	cursor, limit, err := bookmarkPaging(c.QueryParams())
	if err != nil {
		return httputil.NewHTTPError(400, err.Error())
	}
	highlight, err := parseHighlightQuery(c.QueryParams(), weibochat.SearchQuery{})
	if err != nil {
		return httputil.NewHTTPError(400, err.Error())
	}
	ctx := c.Request().Context()
	page, err := h.store.ListBookmarkMessages(ctx, owner, id, cursor, limit)
	if err != nil {
		return bookmarkError(err)
	}
	// Assemble each group in bulk, then restore the page's chronological order.
	groups := map[string][]weibochat.Message{}
	for _, m := range page.Messages {
		groups[m.GroupID] = append(groups[m.GroupID], m)
	}
	responses := map[weibochat.MessageRef]MessageResponse{}
	for group, messages := range groups {
		items, e := h.messageResponses(ctx, owner, group, messages, highlight)
		if e != nil {
			return e
		}
		for _, item := range items {
			responses[weibochat.MessageRef{GroupID: item.GroupID, MessageID: item.MessageID}] = item
		}
	}
	items := make([]MessageResponse, 0, len(page.Messages))
	for _, m := range page.Messages {
		items = append(items, responses[weibochat.MessageRef{GroupID: m.GroupID, MessageID: string(m.MessageID)}])
	}
	return c.JSON(200, httputil.NewResp("bookmark messages", BookmarkMessageListResponse{items, optionalCursor(page.NextCursor)}))
}
func (h *Controller) ListBookmarkTags(c *echo.Context) error {
	owner, err := ownerUsername(c)
	if err != nil {
		return err
	}
	tags, err := h.store.ListBookmarkTags(c.Request().Context(), owner)
	if err != nil {
		return err
	}
	return c.JSON(200, httputil.NewResp("bookmark tags", struct {
		Items []string `json:"items"`
	}{tags}))
}
