package weibochat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
)

func TestBookmarkHTTPWorkflow(t *testing.T) {
	e, db := testController(t)
	h := NewController(weibochat.NewPostgresStore(db), nil)
	e.POST("/bookmark-blocks", h.CreateBookmarkBlock)
	e.GET("/bookmark-blocks", h.ListBookmarkBlocks)
	e.GET("/bookmark-blocks/:block_id", h.GetBookmarkBlock)
	e.PATCH("/bookmark-blocks/:block_id", h.UpdateBookmarkBlock)
	e.DELETE("/bookmark-blocks/:block_id", h.DeleteBookmarkBlock)
	e.GET("/bookmark-blocks/:block_id/messages", h.ListBookmarkMessages)
	e.POST("/bookmark-blocks/:block_id/messages", h.AddBookmarkMessages)
	e.DELETE("/bookmark-blocks/:block_id/messages/:group_id/:message_id", h.RemoveBookmarkMessage)
	e.GET("/bookmark-tags", h.ListBookmarkTags)
	e.GET("/groups/:group_id/messages/:message_id/bookmark-blocks", h.ListMessageBookmarkBlocks)
	require.NoError(t, db.Create(&[]weibochat.Message{{GroupID: "1", MessageID: "1", Text: "中文搜索", Raw: []byte(`{}`)}, {GroupID: "2", MessageID: "2", Text: "另一群", Raw: []byte(`{}`)}}).Error)
	request := func(method, path, body string, status int) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		e.ServeHTTP(rec, req)
		require.Equal(t, status, rec.Code, rec.Body.String())
		return rec
	}
	rec := request(http.MethodPost, "/bookmark-blocks", `{"title":" 讨论 ","tags":[" 技术 ","技术"],"messages":[{"group_id":"1","message_id":"1"}]}`, 201)
	var created struct {
		Data BookmarkBlockResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	require.Equal(t, "讨论", created.Data.Title)
	require.Equal(t, []string{"技术"}, created.Data.Tags)
	id := created.Data.ID
	rec = request(http.MethodGet, "/groups/1/messages", "", 200)
	var list struct {
		Data MessageListResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &list))
	require.EqualValues(t, 1, list.Data.Items[0].BookmarkCount)
	request(http.MethodPost, "/bookmark-blocks/"+id+"/messages", `{"messages":[{"group_id":"2","message_id":"2"}]}`, 204)
	rec = request(http.MethodGet, "/bookmark-blocks/"+id+"/messages?highlight_q=中文", "", 200)
	var page struct {
		Data BookmarkMessageListResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	require.Len(t, page.Data.Items, 2)
	require.Len(t, page.Data.Items[0].TextMatches, 1)
	rec = request(http.MethodGet, "/groups/1/messages/1/bookmark-blocks", "", 200)
	var blocks struct {
		Data BookmarkBlockListResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &blocks))
	require.Len(t, blocks.Data.Items, 1)
	request(http.MethodGet, "/bookmark-blocks?cursor=broken", "", 400)
	request(http.MethodGet, "/bookmark-blocks/"+id+"/messages?limit=0", "", 400)
	request(http.MethodPatch, "/bookmark-blocks/"+id, `{"tags":[]}`, 200)
	request(http.MethodPost, "/bookmark-blocks/"+id+"/messages", `{"messages":[]}`, 400)
	request(http.MethodPost, "/bookmark-blocks", `{"messages":[{"group_id":"1","message_id":"999"}]}`, 404)
	request(http.MethodGet, "/bookmark-blocks/not-uuid", "", 400)
	// A real block owned by somebody else must be indistinguishable from missing.
	other, err := h.store.CreateBookmarkBlock(t.Context(), "other", "private", nil, []weibochat.MessageRef{{GroupID: "1", MessageID: "1"}})
	require.NoError(t, err)
	request(http.MethodGet, "/bookmark-blocks/"+other.ID, "", 404)
	request(http.MethodPatch, "/bookmark-blocks/"+other.ID, `{"title":"attack"}`, 404)
	request(http.MethodDelete, "/bookmark-blocks/"+other.ID, "", 404)
	request(http.MethodPost, "/bookmark-blocks/"+other.ID+"/messages", `{"messages":[{"group_id":"1","message_id":"1"}]}`, 404)
	request(http.MethodGet, "/bookmark-blocks/"+other.ID+"/messages", "", 404)
	request(http.MethodDelete, "/bookmark-blocks/"+id+"/messages/1/1", "", 204)
	request(http.MethodDelete, "/bookmark-blocks/"+id, "", 204)
	rec = request(http.MethodGet, "/bookmark-tags", "", 200)
	require.Contains(t, rec.Body.String(), `"items":[]`)
}
