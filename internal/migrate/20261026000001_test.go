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

func TestWeiboFileMigrationRetriesWithoutChangingObservations(t *testing.T) {
	dsn := os.Getenv("WEIBOCHAT_TEST_DSN")
	if dsn == "" {
		t.Skip("set WEIBOCHAT_TEST_DSN for PostgreSQL integration tests")
	}
	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := fmt.Sprintf("chat_files_test_%d", time.Now().UnixNano())
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
	require.NoError(t, db.AutoMigrate(&weibochat.Message{}, &weibochat.Asset{}, &weibochat.ReadState{}, &weibochat.BookmarkBlock{}, &weibochat.BookmarkMessage{}, &weibochat.BookmarkTag{}))
	for index, name := range []string{"七日丝_261025.pdf", "other.zip", "second name.pdf"} {
		fid := 123 + index
		if index == 2 {
			fid = 123 // One shared resource can have different per-message names.
		}
		raw := fmt.Sprintf(`{"gid":1,"id":%d,"type":321,"media_type":5,"content":%q,"fids":[%d]}`, index+1, name, fid)
		message, err := weibochat.ParseMessage([]byte(raw))
		require.NoError(t, err)
		message.Content.Assets = nil
		message.Text = "original archived text"
		message.FetchedAt = time.Now().UTC()
		require.NoError(t, db.Create(&message).Error)
		require.NoError(t, db.Exec(`UPDATE weibo_chat_message SET content='{"links":[{"url":"https://example.com/keep"}],"assets":[{"kind":"image","asset_key":"pic/keep"}],"future_field":{"keep":true}}'::jsonb WHERE message_id=?`, message.MessageID).Error)
	}
	require.NoError(t, db.Create(&weibochat.Asset{GroupID: "1", AssetKey: "file/123", Kind: weibochat.AssetFile, Status: weibochat.AssetStored, ObjectKey: "keep.pdf", ContentType: "application/pdf", ByteSize: 42}).Error)
	store := weibochat.NewPostgresStore(db)
	block, err := store.CreateBookmarkBlock(t.Context(), "reader", "saved", nil, []weibochat.MessageRef{{GroupID: "1", MessageID: "1"}})
	require.NoError(t, err)
	require.NoError(t, db.Create(&weibochat.ReadState{OwnerUsername: "reader", GroupID: "1", ReadThroughMessageID: new("2"), UpdatedAt: time.Now().UTC()}).Error)
	var before []weibochat.Message
	require.NoError(t, db.Order("message_id").Find(&before).Error)
	// Fail after patching message 2, then let the retry repair its missing asset.
	require.NoError(t, db.Exec("ALTER TABLE weibo_chat_asset ADD CONSTRAINT fail_pending_file CHECK (asset_key <> 'file/124')").Error)
	require.Error(t, migrateWeiboFileReferences(db, zap.NewNop()))
	var second weibochat.Message
	require.NoError(t, db.Where("message_id = '2'").First(&second).Error)
	require.Len(t, second.Content.Assets, 2)
	var missing int64
	require.NoError(t, db.Model(&weibochat.Asset{}).Where("asset_key = 'file/124'").Count(&missing).Error)
	require.Zero(t, missing)
	require.NoError(t, db.Exec("ALTER TABLE weibo_chat_asset DROP CONSTRAINT fail_pending_file").Error)
	require.NoError(t, migrateWeiboFileReferences(db, zap.NewNop()))
	require.NoError(t, migrateWeiboFileReferences(db, zap.NewNop()))
	var rows []weibochat.Message
	require.NoError(t, db.Order("message_id").Find(&rows).Error)
	for index, row := range rows {
		require.Equal(t, before[index].Text, row.Text)
		require.JSONEq(t, string(before[index].Raw), string(row.Raw))
		require.Equal(t, before[index].FetchedAt, row.FetchedAt)
		require.Equal(t, before[index].SenderID, row.SenderID)
		require.Equal(t, before[index].EventType, row.EventType)
		require.Len(t, row.Content.Assets, 2)
		parsed, err := weibochat.ParseMessage(row.Raw)
		require.NoError(t, err)
		require.Equal(t, parsed.Content.Assets[0], row.Content.Assets[1])
		var unchanged string
		require.NoError(t, db.Raw("SELECT (content - 'assets')::text FROM weibo_chat_message WHERE message_id=?", row.MessageID).Scan(&unchanged).Error)
		require.JSONEq(t, `{"links":[{"url":"https://example.com/keep"}],"future_field":{"keep":true}}`, unchanged)
	}
	var assets []weibochat.Asset
	require.NoError(t, db.Order("asset_key").Find(&assets).Error)
	require.Len(t, assets, 2)
	require.Equal(t, weibochat.AssetStored, assets[0].Status)
	require.Equal(t, "keep.pdf", assets[0].ObjectKey)
	require.EqualValues(t, 42, assets[0].ByteSize)
	require.Equal(t, weibochat.AssetPending, assets[1].Status)
	require.Equal(t, weibochat.AssetFile, assets[1].Kind)
	bookmarks, err := store.ListBookmarkMessages(t.Context(), "reader", block.ID, "", 50)
	require.NoError(t, err)
	require.Len(t, bookmarks.Messages, 1)
	var state weibochat.ReadState
	require.NoError(t, db.First(&state).Error)
	require.Equal(t, new("2"), state.ReadThroughMessageID)
	require.True(t, registeredMigration(20261026000001).Auto)
}
