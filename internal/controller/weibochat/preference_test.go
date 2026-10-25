package weibochat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/stretchr/testify/require"
)

func TestPreferenceAPIAndMessageAssembly(t *testing.T) {
	e, db := testController(t)
	seedMessages(t, db)
	patch := func(uid, body string, status int) SenderResponse {
		t.Helper()
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPatch, "/users/"+uid+"/preference", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		e.ServeHTTP(rec, req)
		require.Equal(t, status, rec.Code, rec.Body.String())
		var response httputil.Resp[SenderResponse]
		if status == http.StatusOK {
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		}
		return response.Data
	}
	result := patch("10", `{"is_starred":true}`, http.StatusOK)
	require.True(t, result.IsStarred)
	require.Equal(t, "latest name", result.ScreenName)
	result = patch("10", `{"is_blocked":true}`, http.StatusOK)
	require.True(t, result.IsStarred)
	require.True(t, result.IsBlocked)
	// Missing profiles must still carry preferences and the correct UID.
	result = patch("30", `{"is_starred":true}`, http.StatusOK)
	require.True(t, result.IsStarred)
	require.Equal(t, "30", result.UID)
	require.Empty(t, result.ScreenName)
	require.NoError(t, db.Create(&weibochat.UserPreference{OwnerUsername: "another-user", UID: "20", IsBlocked: true}).Error)
	var page httputil.Resp[MessageListResponse]
	rec := get(t, e, "/groups/1/messages", http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	require.Len(t, page.Data.Items, 4)
	require.True(t, page.Data.Items[0].IsSenderBlocked)
	require.True(t, page.Data.Items[0].Sender.IsBlocked)
	require.False(t, page.Data.Items[1].IsSenderBlocked)
	require.True(t, page.Data.Items[2].IsSenderStarred)
	require.Nil(t, page.Data.Items[2].Sender)
	rec = get(t, e, "/groups/1/messages?starred_only=true", http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	require.Len(t, page.Data.Items, 1)
	require.Equal(t, "101", page.Data.Items[0].MessageID)
	rec = get(t, e, "/groups/1/messages?q=image", http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	require.Empty(t, page.Data.Items)
	// highlight_q changes presentation only, including for blocked messages.
	rec = get(t, e, "/groups/1/messages?highlight_q=image", http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	require.Len(t, page.Data.Items, 4)
	rec = get(t, e, "/groups/1/messages?q=%20%20", http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	require.Len(t, page.Data.Items, 4)
	var context httputil.Resp[MessageContextResponse]
	rec = get(t, e, "/groups/1/messages/95/context", http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &context))
	require.True(t, context.Data.Items[0].IsSenderBlocked)
	var preferences httputil.Resp[UserPreferenceListResponse]
	rec = get(t, e, "/user-preferences?starred=true&limit=1", http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &preferences))
	require.Equal(t, "10", *preferences.Data.NextAfterUID)
	rec = get(t, e, "/user-preferences?starred=true&limit=1&after_uid=10", http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &preferences))
	require.Nil(t, preferences.Data.NextAfterUID)
	require.Equal(t, "30", preferences.Data.Items[0].UID)
	rec = get(t, e, "/user-preferences?blocked=true", http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &preferences))
	require.Len(t, preferences.Data.Items, 1)
	require.Equal(t, "10", preferences.Data.Items[0].UID)
	patch("10", `{"is_blocked":false}`, http.StatusOK)
	rec = get(t, e, "/groups/1/messages?starred_only=true&before_message_id=101&limit=1", http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	require.Len(t, page.Data.Items, 1)
	require.Equal(t, "95", page.Data.Items[0].MessageID)
	for _, body := range []string{`{}`, `{"is_starred":null}`, `{"is_blocked":"true"}`} {
		patch("10", body, http.StatusBadRequest)
	}
	patch("invalid", `{"is_starred":true}`, http.StatusBadRequest)
	for _, path := range []string{"/user-preferences?starred=1", "/user-preferences?blocked=", "/user-preferences?starred=true&starred=false", "/user-preferences?limit=0", "/user-preferences?after_uid=foo", "/groups/1/messages?starred_only=yes", "/groups/1/messages/95/context?starred_only=false"} {
		get(t, e, path, http.StatusBadRequest)
	}
}

func TestBookmarkMessagesPreserveReaderStateAcrossGroups(t *testing.T) {
	e, db := testController(t)
	seedMessages(t, db)
	store := weibochat.NewPostgresStore(db)
	h := NewController(store, nil)
	e.GET("/bookmark-blocks/:block_id/messages", h.ListBookmarkMessages)
	refs := []weibochat.MessageRef{{GroupID: "1", MessageID: "101"}, {GroupID: "2", MessageID: "900"}}
	block, err := store.CreateBookmarkBlock(t.Context(), "test-user", "cross group", nil, refs)
	require.NoError(t, err)
	_, err = store.CreateBookmarkBlock(t.Context(), "test-user", "another block", nil, refs[:1])
	require.NoError(t, err)
	_, err = store.CreateBookmarkBlock(t.Context(), "other-owner", "private block", nil, refs)
	require.NoError(t, err)
	yes := true
	_, err = store.UpdateUserPreference(t.Context(), "test-user", "30", weibochat.UserPreferencePatch{IsStarred: &yes, IsBlocked: &yes})
	require.NoError(t, err)
	_, err = store.UpdateUserPreference(t.Context(), "test-user", "40", weibochat.UserPreferencePatch{IsStarred: &yes})
	require.NoError(t, err)
	_, err = store.UpdateUserPreference(t.Context(), "other-owner", "40", weibochat.UserPreferencePatch{IsBlocked: &yes})
	require.NoError(t, err)
	var page httputil.Resp[BookmarkMessageListResponse]
	rec := get(t, e, "/bookmark-blocks/"+block.ID+"/messages", http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	require.Len(t, page.Data.Items, 2)
	byGroup := make(map[string]MessageResponse)
	for _, item := range page.Data.Items {
		byGroup[item.GroupID] = item
	}
	unknown := byGroup["1"]
	require.Equal(t, "101", unknown.MessageID)
	require.EqualValues(t, 2, unknown.BookmarkCount)
	require.Nil(t, unknown.Sender)
	require.True(t, unknown.IsSenderStarred)
	require.True(t, unknown.IsSenderBlocked)
	require.Equal(t, "unknown", unknown.Text) // Blocking is a display preference; bookmarks stay present.
	known := byGroup["2"]
	require.EqualValues(t, 1, known.BookmarkCount)
	require.True(t, known.IsSenderStarred)
	require.False(t, known.IsSenderBlocked)
	require.NotNil(t, known.Sender)
	require.True(t, known.Sender.IsStarred)
	require.False(t, known.Sender.IsBlocked)
}
