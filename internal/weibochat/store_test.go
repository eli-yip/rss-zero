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
	require.NoError(t, db.AutoMigrate(&BookmarkBlock{}, &BookmarkMessage{}, &BookmarkTag{}, &ReadState{}, &GroupMetadata{}, &Message{}, &User{}, &UserPreference{}, &SyncState{}, &Asset{}))
	require.NoError(t, db.AutoMigrate(&BookmarkBlock{}, &BookmarkMessage{}, &BookmarkTag{}, &ReadState{}, &GroupMetadata{}, &Message{}, &User{}, &UserPreference{}, &SyncState{}, &Asset{}))
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
	require.Equal(t, []MessageID{"9007199254740993", "18446744073709551616"}, messageIDs(page.Messages))
	require.True(t, page.HasMore)
	require.Empty(t, page.Messages[0].Raw)
	require.Equal(t, &unknown, page.Messages[0].MediaType)
	require.NotEmpty(t, page.Messages[0].ParseError)

	// A new latest message does not shift the following cursor page.
	inserted := Message{GroupID: "1", MessageID: "18446744073709551617", Raw: []byte(`{}`), FetchedAt: start}
	_, err = store.SaveMessages(ctx, []Message{inserted})
	require.NoError(t, err)
	page, err = store.ListMessages(ctx, "1", MessageFilter{BeforeMessageID: page.Messages[0].MessageID, Limit: 2})
	require.NoError(t, err)
	require.Equal(t, []MessageID{"10", "11"}, messageIDs(page.Messages))
	require.True(t, page.HasMore)
	page, err = store.ListMessages(ctx, "1", MessageFilter{BeforeMessageID: page.Messages[0].MessageID, Limit: 2})
	require.NoError(t, err)
	require.Equal(t, []MessageID{"9"}, messageIDs(page.Messages))
	require.False(t, page.HasMore)

	for _, tc := range []struct {
		name   string
		filter MessageFilter
		want   []MessageID
	}{
		{"senders", MessageFilter{SenderIDs: []string{"10", "30"}}, []MessageID{"9", "11", "9007199254740993"}},
		{"time bounds", MessageFilter{SentFrom: &start, SentBefore: &end}, []MessageID{"9", "10", "18446744073709551616"}},
		{"asset OR", MessageFilter{AssetKinds: []string{"image", "video"}}, []MessageID{"9", "18446744073709551616"}},
		{"sticker distinct", MessageFilter{AssetKinds: []string{"sticker"}}, []MessageID{"10"}},
		{"combined", MessageFilter{SenderIDs: []string{"10", "20"}, SentFrom: &start, SentBefore: &end, AssetKinds: []string{"video", "sticker"}}, []MessageID{"9", "10"}},
		{"no match", MessageFilter{SenderIDs: []string{"40"}}, []MessageID{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.filter.Limit = 50
			page, err := store.ListMessages(ctx, "1", tc.filter)
			require.NoError(t, err)
			require.Equal(t, tc.want, messageIDs(page.Messages))
			require.False(t, page.HasMore)
		})
	}

	filtered := MessageFilter{AssetKinds: []string{"image"}, Limit: 1}
	page, err = store.ListMessages(ctx, "1", filtered)
	require.NoError(t, err)
	require.Equal(t, []MessageID{"18446744073709551616"}, messageIDs(page.Messages))
	filtered.BeforeMessageID = page.Messages[0].MessageID
	page, err = store.ListMessages(ctx, "1", filtered)
	require.NoError(t, err)
	require.Equal(t, []MessageID{"9"}, messageIDs(page.Messages))
	require.False(t, page.HasMore)
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

func TestSearchLiteralTermsAndSQLHighlightAgreement(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	texts := []string{`🙂数据库 HELLO world 100% a_b C:\tmp!`, `数据库 hello new world 100X axb C:tmp!`, `数据库 HELLO world 100% a_b C:\tmp!`, `Éé`}
	for i, text := range texts {
		_, err := store.SaveMessages(ctx, []Message{{GroupID: "1", MessageID: MessageID(fmt.Sprint(i + 1)), SenderID: "10", SentAt: &now, Text: text, Raw: []byte(`{}`), FetchedAt: now}})
		require.NoError(t, err)
	}
	for _, tc := range []struct {
		q    string
		want []MessageID
	}{
		{`数据库 "hello world"`, []MessageID{"1", "3"}},
		{`100% a_b C:\tmp!`, []MessageID{"1", "3"}},
		{`hello`, []MessageID{"1", "2", "3"}},
		{`é`, []MessageID{"4"}},
	} {
		q, err := ParseSearchQuery(tc.q)
		require.NoError(t, err)
		page, err := store.ListMessages(ctx, "1", MessageFilter{Search: q, Limit: 50})
		require.NoError(t, err)
		require.Equal(t, tc.want, messageIDs(page.Messages))
		for _, message := range page.Messages {
			require.NotEmpty(t, q.FindMatches(message.Text))
		}
	}
	q, err := ParseSearchQuery(`数据库 "hello world"`)
	require.NoError(t, err)
	filter := MessageFilter{Search: q, SenderIDs: []string{"10"}, SentFrom: &now, Limit: 1}
	page, err := store.ListMessages(ctx, "1", filter)
	require.NoError(t, err)
	require.Equal(t, []MessageID{"3"}, messageIDs(page.Messages))
	filter.BeforeMessageID = page.Messages[0].MessageID
	page, err = store.ListMessages(ctx, "1", filter)
	require.NoError(t, err)
	require.Equal(t, []MessageID{"1"}, messageIDs(page.Messages))
	require.False(t, page.HasMore)
}

func TestBidirectionalPagingAndContext(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	for _, id := range []MessageID{"9", "10", "99", "100", "9007199254740993"} {
		_, err := store.SaveMessages(ctx, []Message{{GroupID: "1", MessageID: id, SenderID: "20", Text: "match", Raw: []byte(`{}`), FetchedAt: now}})
		require.NoError(t, err)
	}
	for _, tc := range []struct {
		filter MessageFilter
		want   []MessageID
		more   bool
	}{
		{MessageFilter{Limit: 2}, []MessageID{"100", "9007199254740993"}, true},
		{MessageFilter{BeforeMessageID: "100", Limit: 2}, []MessageID{"10", "99"}, true},
		{MessageFilter{BeforeMessageID: "10", Limit: 2}, []MessageID{"9"}, false},
		{MessageFilter{BeforeMessageID: "9", Limit: 2}, []MessageID{}, false},
		{MessageFilter{AfterMessageID: "9", Limit: 2}, []MessageID{"10", "99"}, true},
		{MessageFilter{AfterMessageID: "99", Limit: 2}, []MessageID{"100", "9007199254740993"}, false},
		{MessageFilter{AfterMessageID: "9007199254740993", Limit: 2}, []MessageID{}, false},
		{MessageFilter{AfterMessageID: "11", Limit: 2, Search: SearchQuery{Terms: []string{"MATCH"}}, SenderIDs: []string{"20"}}, []MessageID{"99", "100"}, true},
	} {
		page, err := store.ListMessages(ctx, "1", tc.filter)
		require.NoError(t, err)
		require.Equal(t, tc.want, messageIDs(page.Messages))
		require.Equal(t, tc.more, page.HasMore)
	}
	_, err := store.ListMessages(ctx, "1", MessageFilter{BeforeMessageID: "100", AfterMessageID: "9", Limit: 2})
	require.Error(t, err)
	// Refreshing the newer side finds messages inserted after reaching the end.
	_, err = store.SaveMessages(ctx, []Message{{GroupID: "1", MessageID: "9007199254740994", Raw: []byte(`{}`), FetchedAt: now}})
	require.NoError(t, err)
	page, err := store.ListMessages(ctx, "1", MessageFilter{AfterMessageID: "9007199254740993", Limit: 2})
	require.NoError(t, err)
	require.Equal(t, []MessageID{"9007199254740994"}, messageIDs(page.Messages))
	require.False(t, page.HasMore)

	for _, tc := range []struct {
		anchor                MessageID
		before, after         int
		want                  []MessageID
		moreBefore, moreAfter bool
	}{
		{"99", 1, 1, []MessageID{"10", "99", "100"}, true, true},
		{"99", 0, 0, []MessageID{"99"}, true, true},
		{"9", 0, 0, []MessageID{"9"}, false, true},
		{"9007199254740994", 0, 0, []MessageID{"9007199254740994"}, true, false},
		{"99", 20, 20, []MessageID{"9", "10", "99", "100", "9007199254740993", "9007199254740994"}, false, false},
	} {
		result, err := store.GetMessageContext(ctx, "1", tc.anchor, tc.before, tc.after)
		require.NoError(t, err)
		require.Equal(t, tc.anchor, result.AnchorMessageID)
		require.Equal(t, tc.want, messageIDs(result.Messages))
		require.Equal(t, tc.moreBefore, result.HasMoreBefore)
		require.Equal(t, tc.moreAfter, result.HasMoreAfter)
		for _, message := range result.Messages {
			require.Empty(t, message.Raw)
		}
	}
	_, err = store.GetMessageContext(ctx, "2", "99", 20, 20)
	require.ErrorIs(t, err, ErrMessageNotFound)
	_, err = store.GetMessageContext(ctx, "1", "98", 20, 20)
	require.ErrorIs(t, err, ErrMessageNotFound)
}
