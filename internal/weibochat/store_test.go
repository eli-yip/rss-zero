package weibochat

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func testStore(t *testing.T) (*PostgresStore, *gorm.DB) {
	t.Helper()
	dsn := os.Getenv("WEIBOCHAT_TEST_DSN")
	if dsn == "" {
		t.Skip("set WEIBOCHAT_TEST_DSN to run PostgreSQL integration tests")
	}
	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	schema := fmt.Sprintf("weibochat_test_%d", time.Now().UnixNano())
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
	require.NoError(t, db.AutoMigrate(&Message{}, &SyncState{}, &Asset{}))
	require.NoError(t, db.AutoMigrate(&Message{}, &SyncState{}, &Asset{}))
	return NewPostgresStore(db), db
}

func TestStoreFirstObservationAndReplayRepair(t *testing.T) {
	store, db := testStore(t)
	ctx := context.Background()
	message, err := ParseMessage([]byte(`{"gid":1,"id":9007199254740993,"media_type":1,"content":"first","fids":[123]}`))
	require.NoError(t, err)
	message.FetchedAt = time.Now().UTC()
	n, err := store.SaveMessages(ctx, []Message{message})
	require.NoError(t, err)
	require.Equal(t, 1, n)
	// Simulate interruption between message and asset writes.
	require.NoError(t, db.Where("group_id = ?", "1").Delete(&Asset{}).Error)
	changed, err := ParseMessage([]byte(`{"gid":1,"id":9007199254740993,"content":"edited","media_type":1,"fids":[456]}`))
	require.NoError(t, err)
	n, err = store.SaveMessages(ctx, []Message{changed})
	require.NoError(t, err)
	require.Zero(t, n)
	var saved Message
	require.NoError(t, db.First(&saved).Error)
	require.Equal(t, "first", saved.Text)
	require.JSONEq(t, string(message.Raw), string(saved.Raw))
	assets, err := store.ListAssets(ctx, "1", "", 20)
	require.NoError(t, err)
	require.Len(t, assets, 1)
	require.Equal(t, "file/123", assets[0].AssetKey)
	assets[0].Status = AssetStored
	assets[0].ObjectKey = "weibo-chat/1/file/123.png"
	require.NoError(t, store.SaveAssetResult(ctx, &assets[0]))
	_, err = store.SaveMessages(ctx, []Message{message})
	require.NoError(t, err)
	pending, err := store.ListAssets(ctx, "1", "", 20)
	require.NoError(t, err)
	require.Empty(t, pending)
}

func TestStorePartialFailureAndState(t *testing.T) {
	store, db := testStore(t)
	ctx := context.Background()
	state, err := store.GetState(ctx, "1")
	require.NoError(t, err)
	require.Nil(t, state)
	now := time.Now().UTC()
	state = &SyncState{GroupID: "1", InitializedAt: now, SyncedThroughMessageID: "10", BackfillBeforeMessageID: "2"}
	require.NoError(t, store.SaveState(ctx, state))
	require.NoError(t, db.Exec("ALTER TABLE weibo_chat_asset ADD CONSTRAINT test_failure CHECK (asset_key <> 'file/123')").Error)
	message, err := ParseMessage([]byte(`{"gid":1,"id":11,"media_type":1,"fids":[123]}`))
	require.NoError(t, err)
	message.FetchedAt = now
	_, err = store.SaveMessages(ctx, []Message{message})
	require.Error(t, err)
	state, err = store.GetState(ctx, "1")
	require.NoError(t, err)
	require.Equal(t, MessageID("10"), state.SyncedThroughMessageID)
	require.NoError(t, db.Exec("ALTER TABLE weibo_chat_asset DROP CONSTRAINT test_failure").Error)
	n, err := store.SaveMessages(ctx, []Message{message})
	require.NoError(t, err)
	require.Zero(t, n)
	assets, err := store.ListAssets(ctx, "1", "", 20)
	require.NoError(t, err)
	require.Len(t, assets, 1)
	state.SyncedThroughMessageID = "11"
	state.BackfillBeforeMessageID = ""
	state.HistoryExhaustedAt = &now
	require.NoError(t, store.SaveState(ctx, state))
	state, err = store.GetState(ctx, "1")
	require.NoError(t, err)
	require.Empty(t, state.BackfillBeforeMessageID)
	require.NotNil(t, state.HistoryExhaustedAt)
}
