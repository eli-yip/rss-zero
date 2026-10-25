package weibochat

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPreferenceIsolationAndPartialUpdates(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	yes, no := true, false
	p, err := store.UpdateUserPreference(ctx, "alice", "10", UserPreferencePatch{IsStarred: &yes})
	require.NoError(t, err)
	require.True(t, p.IsStarred)
	require.False(t, p.IsBlocked)
	p, err = store.UpdateUserPreference(ctx, "alice", "10", UserPreferencePatch{IsBlocked: &yes})
	require.NoError(t, err)
	require.True(t, p.IsStarred)
	require.True(t, p.IsBlocked)
	p, err = store.UpdateUserPreference(ctx, "alice", "10", UserPreferencePatch{IsBlocked: &no})
	require.NoError(t, err)
	require.True(t, p.IsStarred)
	require.False(t, p.IsBlocked)
	_, err = store.UpdateUserPreference(ctx, "bob", "10", UserPreferencePatch{IsBlocked: &yes})
	require.NoError(t, err)
	prefs, err := store.GetUserPreferences(ctx, "alice", []string{"10"})
	require.NoError(t, err)
	require.False(t, prefs["10"].IsBlocked)
	_, err = store.UpdateUserPreference(ctx, "alice", "9", UserPreferencePatch{IsStarred: &yes})
	require.NoError(t, err)
	page, err := store.ListUserPreferences(ctx, "alice", UserPreferenceFilter{Starred: &yes, Limit: 1})
	require.NoError(t, err)
	require.True(t, page.HasMore)
	require.Equal(t, "9", page.Items[0].UID)
	page, err = store.ListUserPreferences(ctx, "alice", UserPreferenceFilter{Starred: &yes, AfterUID: "9", Limit: 1})
	require.NoError(t, err)
	require.False(t, page.HasMore)
	require.Equal(t, "10", page.Items[0].UID)
}

func TestStarredAndBlockedMessageQueries(t *testing.T) {
	store, db := testStore(t)
	ctx := context.Background()
	yes := true
	for _, m := range []Message{
		{GroupID: "1", MessageID: "9", SenderID: "10", Text: "中文"},
		{GroupID: "1", MessageID: "10", SenderID: "20", Text: "中文"},
		{GroupID: "1", MessageID: "11", SenderID: "30", Text: "中文"},
		{GroupID: "2", MessageID: "12", SenderID: "10", Text: "中文"},
	} {
		m.Raw = json.RawMessage(`{}`)
		m.FetchedAt = time.Now()
		require.NoError(t, db.Create(&m).Error)
	}
	_, err := store.UpdateUserPreference(ctx, "alice", "10", UserPreferencePatch{IsStarred: &yes})
	require.NoError(t, err)
	_, err = store.UpdateUserPreference(ctx, "alice", "20", UserPreferencePatch{IsStarred: &yes, IsBlocked: &yes})
	require.NoError(t, err)
	q, _ := ParseSearchQuery("中文")
	for _, tc := range []struct {
		group  string
		filter MessageFilter
		ids    []MessageID
	}{
		{"1", MessageFilter{OwnerUsername: "alice", Limit: 10}, []MessageID{"9", "10", "11"}},
		{"1", MessageFilter{OwnerUsername: "alice", Search: q, Limit: 10}, []MessageID{"9", "11"}},
		{"1", MessageFilter{OwnerUsername: "bob", Search: q, Limit: 10}, []MessageID{"9", "10", "11"}},
		{"1", MessageFilter{OwnerUsername: "alice", StarredOnly: true, Limit: 10}, []MessageID{"9"}},
		{"2", MessageFilter{OwnerUsername: "alice", StarredOnly: true, Limit: 10}, []MessageID{"12"}},
		{"1", MessageFilter{OwnerUsername: "alice", StarredOnly: true, AfterMessageID: "9", Limit: 1}, []MessageID{}},
		{"1", MessageFilter{OwnerUsername: "alice", StarredOnly: true, SenderIDs: []string{"30"}, Limit: 10}, []MessageID{}},
	} {
		page, err := store.ListMessages(ctx, tc.group, tc.filter)
		require.NoError(t, err)
		ids := []MessageID{}
		for _, m := range page.Messages {
			ids = append(ids, m.MessageID)
		}
		require.Equal(t, tc.ids, ids)
	}
	context, err := store.GetMessageContext(ctx, "1", "10", 1, 1)
	require.NoError(t, err)
	require.Len(t, context.Messages, 3)
}
