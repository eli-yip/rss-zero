package migrate

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestWeiboGroupEventsMigrationRetriesAndPreservesObservations(t *testing.T) {
	dsn := os.Getenv("WEIBOCHAT_TEST_DSN")
	if dsn == "" {
		t.Skip("set WEIBOCHAT_TEST_DSN for PostgreSQL integration tests")
	}
	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := fmt.Sprintf("group_events_test_%d", time.Now().UnixNano())
	require.NoError(t, base.Exec("CREATE SCHEMA "+schema).Error)
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		conn, _ := db.DB()
		_ = conn.Close()
		_ = base.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		conn, _ = base.DB()
		_ = conn.Close()
	})
	require.NoError(t, db.AutoMigrate(&weibochat.Message{}, &weibochat.User{}, &weibochat.ReadState{}, &weibochat.BookmarkBlock{}, &weibochat.BookmarkMessage{}, &weibochat.BookmarkTag{}))
	for index, raw := range []string{
		`{"gid":1,"id":1,"type":322,"media_type":0,"sub_type":101,"from_uid":42,"template":"{{name.DATA}} 加入了群","data":{"name":{"value":"新人","scheme":"sinaweibo://userinfo?uid=23"}}}`,
		`{"gid":1,"id":2,"type":344,"media_type":0,"sub_type":101,"from_uid":42,"from_user":{"screen_name":"观察者"},"content":"观察者 撤回了一条消息"}`,
		`{"gid":1,"id":3,"type":100,"media_type":0,"sub_type":101,"from_uid":42,"from_user":{"screen_name":"观察者"},"content":"我发布了新微博，快来围观吧！"}`,
		`{"gid":1,"id":4,"type":321,"media_type":0,"sub_type":101,"from_uid":5394206608,"template":"恭喜 {{user.DATA}} 升级至{{level.DATA}}"}`,
		`{"gid":1,"id":5,"type":322,"media_type":0,"sub_type":101,"from_uid":42,"template":"未知通知"}`,
	} {
		message, err := weibochat.ParseMessage([]byte(raw))
		require.NoError(t, err)
		message.SenderID = "42"
		message.EventType = weibochat.EventNone
		message.Content.EventActor = nil
		message.Text = "original archived text"
		message.FetchedAt = time.Now().UTC()
		require.NoError(t, db.Create(&message).Error)
		content := `{"links":[{"url":"https://example.com/preserved"}],"assets":[{"kind":"image","asset_key":"pic/keep"}],"future_field":{"keep":true}}`
		require.NoError(t, db.Exec("UPDATE weibo_chat_message SET content=?::jsonb WHERE message_id=?", content, fmt.Sprint(index+1)).Error)
	}
	require.NoError(t, db.Create(&weibochat.User{UID: "42", ScreenName: "keep user", ProfileObservedAt: time.Now().UTC()}).Error)
	store := weibochat.NewPostgresStore(db)
	block, err := store.CreateBookmarkBlock(t.Context(), "reader", "saved", nil, []weibochat.MessageRef{{GroupID: "1", MessageID: "1"}})
	require.NoError(t, err)
	require.NoError(t, db.Create(&weibochat.ReadState{OwnerUsername: "reader", GroupID: "1", ReadThroughMessageID: new("3"), UpdatedAt: time.Now().UTC()}).Error)
	// Model a legacy schema, then fail after one successful row to verify retry.
	require.NoError(t, db.Exec("ALTER TABLE weibo_chat_message DROP COLUMN event_type").Error)
	require.NoError(t, db.Exec("ALTER TABLE weibo_chat_message ADD CONSTRAINT fail_second_event CHECK (message_id <> '2' OR sender_id <> '')").Error)
	require.Error(t, migrateWeiboGroupEvents(db, zap.NewNop()))
	require.NoError(t, db.Exec("ALTER TABLE weibo_chat_message DROP CONSTRAINT fail_second_event").Error)
	require.NoError(t, migrateWeiboGroupEvents(db, zap.NewNop()))
	require.NoError(t, migrateWeiboGroupEvents(db, zap.NewNop()))
	var rows []weibochat.Message
	require.NoError(t, db.Order("message_id").Find(&rows).Error)
	require.Len(t, rows, 5)
	for index, row := range rows {
		require.Equal(t, "original archived text", row.Text)
		var unchanged string
		require.NoError(t, db.Raw("SELECT (content - 'event_actor')::text FROM weibo_chat_message WHERE message_id=?", row.MessageID).Scan(&unchanged).Error)
		require.JSONEq(t, `{"links":[{"url":"https://example.com/preserved"}],"assets":[{"kind":"image","asset_key":"pic/keep"}],"future_field":{"keep":true}}`, unchanged)
		parsed, err := weibochat.ParseMessage(row.Raw)
		require.NoError(t, err)
		require.Equal(t, parsed.EventType, row.EventType)
		if index < 4 {
			require.Empty(t, row.SenderID)
			require.Equal(t, parsed.Content.EventActor, row.Content.EventActor)
		} else {
			require.Equal(t, "42", row.SenderID)
			require.Nil(t, row.Content.EventActor)
		}
	}
	user, err := store.GetUser(t.Context(), "42")
	require.NoError(t, err)
	require.Equal(t, "keep user", user.ScreenName)
	bookmarks, err := store.ListBookmarkMessages(t.Context(), "reader", block.ID, "", 50)
	require.NoError(t, err)
	require.Len(t, bookmarks.Messages, 1)
	var state weibochat.ReadState
	require.NoError(t, db.First(&state).Error)
	require.Equal(t, new("3"), state.ReadThroughMessageID)
	require.True(t, registeredMigration(20261026000000).Auto)
}
