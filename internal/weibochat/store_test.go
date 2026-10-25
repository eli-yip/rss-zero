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
	require.NoError(t, db.AutoMigrate(&Message{}, &User{}, &SyncState{}, &Asset{}))
	require.NoError(t, db.AutoMigrate(&Message{}, &User{}, &SyncState{}, &Asset{}))
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

func TestListMessagesFiltersAndPagination(t *testing.T) {
	store, db := testStore(t)
	ctx := context.Background()
	start := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	middle, end := start.Add(time.Hour), start.Add(2*time.Hour)
	unknown := 99
	messages := []Message{
		{GroupID: "1", MessageID: "9", SenderID: "10", SentAt: &start, Content: MessageContent{Assets: []AssetRef{{Kind: "image", AssetKey: "pic/1"}, {Kind: "video", AssetKey: "file/2"}}}},
		{GroupID: "1", MessageID: "10", SenderID: "20", SentAt: &middle, Content: MessageContent{Assets: []AssetRef{{Kind: "sticker", AssetKey: "pic/3"}}}},
		{GroupID: "1", MessageID: "11", SenderID: "10", SentAt: &end},
		{GroupID: "1", MessageID: "9007199254740993", SenderID: "30", MediaType: &unknown, ParseError: "incomplete content"},
		{GroupID: "1", MessageID: "18446744073709551616", SenderID: "20", SentAt: &middle, Content: MessageContent{Assets: []AssetRef{{Kind: "image", AssetKey: "pic/4"}}}},
		{GroupID: "2", MessageID: "100", SenderID: "40", SentAt: &middle},
	}
	for i := range messages {
		messages[i].Raw = []byte(`{"private":"raw"}`)
		messages[i].FetchedAt = start
	}
	_, err := store.SaveMessages(ctx, messages)
	require.NoError(t, err)

	page, err := store.ListMessages(ctx, "1", MessageFilter{Limit: 2})
	require.NoError(t, err)
	require.Equal(t, []MessageID{"18446744073709551616", "9007199254740993"}, messageIDs(page.Messages))
	require.Equal(t, MessageID("9007199254740993"), page.NextBeforeMessageID)
	require.Empty(t, page.Messages[0].Raw)
	require.Equal(t, &unknown, page.Messages[1].MediaType)
	require.NotEmpty(t, page.Messages[1].ParseError)

	// A new latest message does not shift the following cursor page.
	inserted := Message{GroupID: "1", MessageID: "18446744073709551617", Raw: []byte(`{}`), FetchedAt: start}
	_, err = store.SaveMessages(ctx, []Message{inserted})
	require.NoError(t, err)
	page, err = store.ListMessages(ctx, "1", MessageFilter{BeforeMessageID: page.NextBeforeMessageID, Limit: 2})
	require.NoError(t, err)
	require.Equal(t, []MessageID{"11", "10"}, messageIDs(page.Messages))
	require.Equal(t, MessageID("10"), page.NextBeforeMessageID)
	page, err = store.ListMessages(ctx, "1", MessageFilter{BeforeMessageID: page.NextBeforeMessageID, Limit: 2})
	require.NoError(t, err)
	require.Equal(t, []MessageID{"9"}, messageIDs(page.Messages))
	require.Empty(t, page.NextBeforeMessageID)

	for _, tc := range []struct {
		name   string
		filter MessageFilter
		want   []MessageID
	}{
		{"senders", MessageFilter{SenderIDs: []string{"10", "30"}}, []MessageID{"9007199254740993", "11", "9"}},
		{"time bounds", MessageFilter{SentFrom: &start, SentBefore: &end}, []MessageID{"18446744073709551616", "10", "9"}},
		{"asset OR", MessageFilter{AssetKinds: []string{"image", "video"}}, []MessageID{"18446744073709551616", "9"}},
		{"sticker distinct", MessageFilter{AssetKinds: []string{"sticker"}}, []MessageID{"10"}},
		{"combined", MessageFilter{SenderIDs: []string{"10", "20"}, SentFrom: &start, SentBefore: &end, AssetKinds: []string{"video", "sticker"}}, []MessageID{"10", "9"}},
		{"no match", MessageFilter{SenderIDs: []string{"40"}}, []MessageID{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.filter.Limit = 50
			page, err := store.ListMessages(ctx, "1", tc.filter)
			require.NoError(t, err)
			require.Equal(t, tc.want, messageIDs(page.Messages))
			require.Empty(t, page.NextBeforeMessageID)
		})
	}

	filtered := MessageFilter{AssetKinds: []string{"image"}, Limit: 1}
	page, err = store.ListMessages(ctx, "1", filtered)
	require.NoError(t, err)
	require.Equal(t, []MessageID{"18446744073709551616"}, messageIDs(page.Messages))
	filtered.BeforeMessageID = page.NextBeforeMessageID
	page, err = store.ListMessages(ctx, "1", filtered)
	require.NoError(t, err)
	require.Equal(t, []MessageID{"9"}, messageIDs(page.Messages))
	require.Empty(t, page.NextBeforeMessageID)
	for _, limit := range []int{0, -1, 101} {
		_, err := store.ListMessages(ctx, "1", MessageFilter{Limit: limit})
		require.Error(t, err)
	}

	// The auto-migrated index uses the exact numeric ID order of the query.
	var indexDefinition string
	require.NoError(t, db.Raw("SELECT indexdef FROM pg_indexes WHERE schemaname = current_schema() AND indexname = 'idx_weibo_chat_message_order'").Scan(&indexDefinition).Error)
	require.Contains(t, indexDefinition, `group_id, length(message_id) DESC, message_id COLLATE "C" DESC`)
}

func messageIDs(messages []Message) []MessageID {
	ids := make([]MessageID, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.MessageID)
	}
	return ids
}

func TestListGroupsAndGetAssets(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	_, err := store.SaveMessages(ctx, []Message{
		{GroupID: "2", MessageID: "1", Raw: []byte(`{}`), FetchedAt: now, Content: MessageContent{Assets: []AssetRef{{Kind: "image", AssetKey: "pic/1"}, {Kind: "video", AssetKey: "file/2"}}}},
		{GroupID: "10", MessageID: "2", Raw: []byte(`{}`), FetchedAt: now, Content: MessageContent{Assets: []AssetRef{{Kind: "image", AssetKey: "pic/1"}}}},
	})
	require.NoError(t, err)
	require.NoError(t, store.SaveState(ctx, &SyncState{GroupID: "10", InitializedAt: now, LastSyncedAt: &now}))
	require.NoError(t, store.SaveState(ctx, &SyncState{GroupID: "100", InitializedAt: now}))
	groups, err := store.ListGroups(ctx)
	require.NoError(t, err)
	require.Len(t, groups, 3)
	require.Equal(t, Group{GroupID: "2"}, groups[0])
	require.Equal(t, "10", groups[1].GroupID)
	require.True(t, now.Equal(*groups[1].InitializedAt))
	require.True(t, now.Equal(*groups[1].LastSyncedAt))
	require.Equal(t, "100", groups[2].GroupID)
	require.True(t, now.Equal(*groups[2].InitializedAt))
	require.Nil(t, groups[2].LastSyncedAt)

	require.NoError(t, store.SaveAssetResult(ctx, &Asset{GroupID: "2", AssetKey: "pic/1", Status: AssetFailed, LastError: "retry later"}))
	assets, err := store.GetAssets(ctx, "2", []string{"pic/1", "file/2", "missing"})
	require.NoError(t, err)
	require.Len(t, assets, 2)
	byKey := map[string]Asset{}
	for _, asset := range assets {
		require.Equal(t, "2", asset.GroupID)
		byKey[asset.AssetKey] = asset
	}
	require.Equal(t, AssetFailed, byKey["pic/1"].Status)
	require.Equal(t, AssetPending, byKey["file/2"].Status)
	empty, err := store.GetAssets(ctx, "2", nil)
	require.NoError(t, err)
	require.Empty(t, empty)
}
