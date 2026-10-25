package weibochat

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

//go:fix inline
func TestReadStateCursorAndIsolation(t *testing.T) {
	store, db := testStore(t)
	require.NoError(t, db.Create(&[]Message{
		{GroupID: "1", MessageID: "9", Raw: []byte(`{}`)},
		{GroupID: "1", MessageID: "10", Raw: []byte(`{}`)},
		{GroupID: "1", MessageID: "9007199254740993", Raw: []byte(`{}`)},
		{GroupID: "2", MessageID: "20", Raw: []byte(`{}`)},
	}).Error)
	ctx := t.Context()
	state, err := store.GetReadState(ctx, "alice", "1")
	require.NoError(t, err)
	require.Nil(t, state.ReadThroughMessageID)
	require.Equal(t, new("9"), state.FirstUnreadMessageID)
	require.Equal(t, new("9007199254740993"), state.LatestMessageID)
	require.Zero(t, state.Revision)
	state, err = store.UpdateReadState(ctx, "alice", "1", ReadStatePatch{Action: "advance", MessageID: new("10")})
	require.NoError(t, err)
	require.Equal(t, new("10"), state.ReadThroughMessageID)
	require.Equal(t, new("9007199254740993"), state.FirstUnreadMessageID)
	require.EqualValues(t, 1, state.Revision)
	// A delayed earlier page never moves the cursor backwards.
	state, err = store.UpdateReadState(ctx, "alice", "1", ReadStatePatch{Action: "advance", MessageID: new("9"), Revision: 1})
	require.NoError(t, err)
	require.Equal(t, new("10"), state.ReadThroughMessageID)
	state, err = store.UpdateReadState(ctx, "alice", "1", ReadStatePatch{Action: "mark_unread", MessageID: new("10"), Revision: 2})
	require.NoError(t, err)
	require.Equal(t, new("9"), state.ReadThroughMessageID)
	require.Equal(t, new("10"), state.FirstUnreadMessageID)
	_, err = store.UpdateReadState(ctx, "alice", "1", ReadStatePatch{Action: "advance", MessageID: new("9007199254740993"), Revision: 2})
	require.ErrorIs(t, err, ErrReadStateConflict)
	state, err = store.UpdateReadState(ctx, "alice", "1", ReadStatePatch{Action: "mark_unread", MessageID: new("9"), Revision: 3})
	require.NoError(t, err)
	require.Nil(t, state.ReadThroughMessageID)
	// Undo uses the returned revision and can restore a nullable cursor.
	state, err = store.UpdateReadState(ctx, "alice", "1", ReadStatePatch{Action: "restore", MessageID: new("10"), Revision: 4})
	require.NoError(t, err)
	require.Equal(t, new("10"), state.ReadThroughMessageID)
	// Mark all only consumes the explicitly displayed snapshot; a new arrival stays unread.
	require.NoError(t, db.Create(&Message{GroupID: "1", MessageID: "9007199254740994", Raw: []byte(`{}`)}).Error)
	state, err = store.UpdateReadState(ctx, "alice", "1", ReadStatePatch{Action: "mark_all_read", MessageID: new("9007199254740993"), Revision: 5})
	require.NoError(t, err)
	require.Equal(t, new("9007199254740994"), state.FirstUnreadMessageID)
	state, err = store.UpdateReadState(ctx, "alice", "1", ReadStatePatch{Action: "advance", MessageID: new("9007199254740994"), Revision: 6})
	require.NoError(t, err)
	require.Nil(t, state.FirstUnreadMessageID)
	unread, err := store.GroupsWithUnread(ctx, "alice", []string{"1", "2", "3"})
	require.NoError(t, err)
	require.False(t, unread["1"])
	require.True(t, unread["2"])
	require.False(t, unread["3"])
	unread, err = store.GroupsWithUnread(ctx, "bob", []string{"1", "2"})
	require.NoError(t, err)
	require.True(t, unread["1"])
	state, err = store.GetReadState(ctx, "bob", "1")
	require.NoError(t, err)
	require.Zero(t, state.Revision)
	require.Nil(t, state.ReadThroughMessageID)
	state, err = store.UpdateReadState(ctx, "alice", "1", ReadStatePatch{Action: "restore", Revision: 7})
	require.NoError(t, err)
	require.Nil(t, state.ReadThroughMessageID)
	require.Equal(t, new("9"), state.FirstUnreadMessageID)
	_, err = store.UpdateReadState(ctx, "alice", "1", ReadStatePatch{Action: "advance", MessageID: new("20"), Revision: 8})
	require.ErrorIs(t, err, ErrMessageNotFound)
	state, err = store.GetReadState(ctx, "alice", "1")
	require.NoError(t, err)
	require.EqualValues(t, 8, state.Revision)
	empty, err := store.GetReadState(ctx, "alice", "3")
	require.NoError(t, err)
	require.Nil(t, empty.FirstUnreadMessageID)
	require.Nil(t, empty.LatestMessageID)
}

func TestReadStateConcurrentFirstWrites(t *testing.T) {
	store, db := testStore(t)
	require.NoError(t, db.Create(&Message{GroupID: "1", MessageID: "10", Raw: []byte(`{}`)}).Error)
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	start := make(chan struct{})
	for range 2 {
		wg.Go(func() {
			<-start
			_, err := store.UpdateReadState(t.Context(), "alice", "1", ReadStatePatch{Action: "advance", MessageID: new("10")})
			errs <- err
		})
	}
	close(start)
	wg.Wait()
	close(errs)
	successes, conflicts := 0, 0
	for err := range errs {
		if err == nil {
			successes++
		} else if errors.Is(err, ErrReadStateConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	require.Equal(t, 1, successes)
	require.Equal(t, 1, conflicts)
	state, err := store.GetReadState(t.Context(), "alice", "1")
	require.NoError(t, err)
	require.EqualValues(t, 1, state.Revision)
}
