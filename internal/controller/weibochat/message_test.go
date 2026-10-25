package weibochat

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testController(t *testing.T) (*echo.Echo, *gorm.DB) {
	t.Helper()
	dsn := os.Getenv("WEIBOCHAT_TEST_DSN")
	if dsn == "" {
		t.Skip("set WEIBOCHAT_TEST_DSN to run PostgreSQL integration tests")
	}
	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := fmt.Sprintf("weibochat_api_%d", time.Now().UnixNano())
	require.NoError(t, base.Exec("CREATE SCHEMA "+schema).Error)
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		conn, err := db.DB()
		require.NoError(t, err)
		require.NoError(t, conn.Close())
		require.NoError(t, base.Exec("DROP SCHEMA "+schema+" CASCADE").Error)
		conn, err = base.DB()
		require.NoError(t, err)
		require.NoError(t, conn.Close())
	})
	require.NoError(t, db.AutoMigrate(&weibochat.BookmarkBlock{}, &weibochat.BookmarkMessage{}, &weibochat.BookmarkTag{}, &weibochat.Message{}, &weibochat.User{}, &weibochat.SyncState{}, &weibochat.Asset{}))
	h := NewController(weibochat.NewPostgresStore(db), []string{"1", "3", "1"})
	return testRoutes(h), db
}

func testRoutes(h *Controller) *echo.Echo {
	e := echo.New()
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error { c.Set("username", "test-user"); return next(c) }
	})
	e.HTTPErrorHandler = httputil.NewHTTPErrorHandler(zap.NewNop())
	e.GET("/groups", h.ListGroups)
	e.GET("/groups/:group_id/messages", h.ListMessages)
	e.GET("/groups/:group_id/senders", h.ListSenders)
	e.GET("/groups/:group_id/messages/:message_id/context", h.GetMessageContext)
	return e
}

func get(t *testing.T, e *echo.Echo, path string, wantStatus int) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	require.Equal(t, wantStatus, rec.Code, rec.Body.String())
	return rec
}

func seedMessages(t *testing.T, db *gorm.DB) time.Time {
	t.Helper()
	sent := time.Date(2026, 10, 25, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))
	later := sent.Add(time.Hour)
	unknown := 999
	messages := []weibochat.Message{
		{GroupID: "1", MessageID: "95", SenderID: "10", SentAt: &sent, Text: "image", Content: weibochat.MessageContent{Assets: []weibochat.AssetRef{{Kind: "image", AssetKey: "file/1"}, {Kind: "video", AssetKey: "file/2"}, {Kind: "sticker", AssetKey: "file/missing"}}}},
		{GroupID: "1", MessageID: "100", SenderID: "20", SentAt: &later, Text: "partial", ParseError: "private parse error"},
		{GroupID: "1", MessageID: "101", SenderID: "30", MediaType: &unknown, Text: "unknown"},
		{GroupID: "1", MessageID: "102", Text: "system"},
		{GroupID: "2", MessageID: "900", SenderID: "40", Text: "other group"},
	}
	for i := range messages {
		messages[i].Raw = json.RawMessage(`{"private":"raw"}`)
		messages[i].FetchedAt = sent
	}
	require.NoError(t, db.Create(&messages).Error)
	users := []weibochat.User{{UID: "10", ScreenName: "latest name", AvatarSourceURL: "https://private.example/avatar", AvatarStorageProvider: "https://assets.example", AvatarObjectKey: "avatars/a b.jpg", ProfileObservedAt: sent}, {UID: "20", ScreenName: "second", ProfileObservedAt: sent}, {UID: "40", ScreenName: "other", ProfileObservedAt: sent}}
	require.NoError(t, db.Create(&users).Error)
	assets := []weibochat.Asset{{GroupID: "1", AssetKey: "file/1", Kind: "image", Status: weibochat.AssetStored, ObjectKey: "pictures/a b.jpg", StorageProvider: "https://assets.example", ContentType: "image/jpeg", ByteSize: 123}, {GroupID: "1", AssetKey: "file/2", Kind: "video", Status: weibochat.AssetFailed, LastError: "private download error", SourceURL: "https://private.example/file"}}
	require.NoError(t, db.Create(&assets).Error)
	require.NoError(t, db.Create(&weibochat.SyncState{GroupID: "1", InitializedAt: sent, LastSyncedAt: &later}).Error)
	require.NoError(t, db.Create(&weibochat.SyncState{GroupID: "4", InitializedAt: sent}).Error)
	return sent
}

func TestMessageListPaginationAndAssembly(t *testing.T) {
	e, db := testController(t)
	seedMessages(t, db)
	var first httputil.Resp[MessageListResponse]
	rec := get(t, e, "/groups/1/messages?limit=2", http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &first))
	require.Len(t, first.Data.Items, 2)
	require.Equal(t, "101", first.Data.Items[0].MessageID)
	require.Equal(t, "102", first.Data.Items[1].MessageID)
	require.Nil(t, first.Data.Items[0].Sender)
	require.Nil(t, first.Data.Items[0].SentAt)
	require.Equal(t, "parsed", first.Data.Items[0].ParseStatus)
	require.True(t, first.Data.HasMore)
	var next httputil.Resp[MessageListResponse]
	rec = get(t, e, "/groups/1/messages?limit=2&before_message_id=101", http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &next))
	require.Len(t, next.Data.Items, 2)
	require.False(t, next.Data.HasMore)
	require.Equal(t, "partial", next.Data.Items[1].ParseStatus)
	message := next.Data.Items[0]
	require.Equal(t, "95", message.MessageID)
	require.Equal(t, "latest name", message.Sender.ScreenName)
	require.Equal(t, "https://assets.example/avatars/a%20b.jpg", *message.Sender.AvatarURL)
	require.Equal(t, time.UTC, message.SentAt.Location())
	require.Len(t, message.Assets, 3)
	require.Equal(t, "https://assets.example/pictures/a%20b.jpg", *message.Assets[0].URL)
	require.Equal(t, "failed", message.Assets[1].Status)
	require.Nil(t, message.Assets[1].URL)
	require.Equal(t, "pending", message.Assets[2].Status)
	require.NotContains(t, rec.Body.String(), "private")
	require.NotContains(t, rec.Body.String(), "raw")
	require.Contains(t, rec.Body.String(), `"links":[]`)
}

func TestMessageListFiltersAndEmptyResult(t *testing.T) {
	e, db := testController(t)
	seedMessages(t, db)
	path := "/groups/1/messages?sender_id=10&sender_id=20&asset_kind=image&asset_kind=sticker&sent_from=2026-10-25T04:00:00Z&sent_before=2026-10-25T05:00:00Z"
	var result httputil.Resp[MessageListResponse]
	rec := get(t, e, path, http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Len(t, result.Data.Items, 1)
	require.Equal(t, "95", result.Data.Items[0].MessageID)
	// Filtering selects complete messages; it does not remove their other assets.
	require.Len(t, result.Data.Items[0].Assets, 3)
	for _, path := range []string{"/groups/999/messages", "/groups/1/messages?sender_id=40"} {
		rec := get(t, e, path, http.StatusOK)
		require.JSONEq(t, `{"message":"weibo messages","data":{"items":[],"has_more":false}}`, rec.Body.String())
	}
}

func TestMessageParameters(t *testing.T) {
	e := testRoutes(NewController(nil, nil))
	for _, path := range []string{
		"/groups/no/messages", "/groups/0/senders", "/groups/1/messages?sender_id=", "/groups/1/messages?sender_id=01",
		"/groups/1/messages?asset_kind=file", "/groups/1/messages?asset_kind=", "/groups/1/messages?before_message_id=",
		"/groups/1/messages?before_message_id=-1", "/groups/1/messages?limit=0", "/groups/1/messages?limit=101",
		"/groups/1/messages?after_message_id=", "/groups/1/messages?after_message_id=01",
		"/groups/1/messages?before_message_id=1&after_message_id=2", "/groups/1/messages?after_message_id=1&after_message_id=2",
		"/groups/1/messages?limit=1&limit=2", "/groups/1/messages?sent_from=2026-10-25", "/groups/1/messages?sent_before=",
		"/groups/1/messages?sent_from=2026-10-25T00:00:00Z&sent_before=2026-10-25T00:00:00Z",
	} {
		t.Run(path, func(t *testing.T) { get(t, e, path, http.StatusBadRequest) })
	}
}

func TestMessageSearchAndIndependentHighlight(t *testing.T) {
	e, db := testController(t)
	now := time.Now().UTC()
	require.NoError(t, db.Create(&[]weibochat.Message{
		{GroupID: "1", MessageID: "1", Text: "🙂数据库 HELLO world", Raw: []byte(`{}`), FetchedAt: now},
		{GroupID: "1", MessageID: "2", Text: "数据库 hello new world", Raw: []byte(`{}`), FetchedAt: now},
	}).Error)
	for _, tc := range []struct {
		query   string
		count   int
		matches []weibochat.TextMatch
	}{
		{`q=` + url.QueryEscape(`数据库 "hello world"`), 1, []weibochat.TextMatch{{Start: 1, End: 4}, {Start: 5, End: 16}}},
		{`q=` + url.QueryEscape(`数据库 "hello world"`) + `&highlight_q=world`, 1, []weibochat.TextMatch{{Start: 11, End: 16}}},
		{`q=` + url.QueryEscape(`数据库 "hello world"`) + `&highlight_q=`, 1, []weibochat.TextMatch{}},
		{`highlight_q=world`, 2, []weibochat.TextMatch{{Start: 11, End: 16}}},
		{`q=%20%20`, 2, []weibochat.TextMatch{}},
	} {
		rec := get(t, e, "/groups/1/messages?"+tc.query, http.StatusOK)
		var response httputil.Resp[MessageListResponse]
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		require.Len(t, response.Data.Items, tc.count)
		require.Equal(t, tc.matches, response.Data.Items[0].TextMatches)
	}
}

func TestSearchParameters(t *testing.T) {
	e := testRoutes(NewController(nil, nil))
	for _, query := range []string{`q=a&q=b`, `highlight_q=a&highlight_q=b`, `q=%22`, `q=%22%22`, `highlight_q=%22`, `q=` + strings.Repeat("x", 257)} {
		get(t, e, "/groups/1/messages?"+query, http.StatusBadRequest)
	}
}
