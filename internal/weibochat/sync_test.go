package weibochat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type memoryStore struct {
	state      *SyncState
	messages   map[MessageID]Message
	saveError  error
	stateError error
}

func (s *memoryStore) GetState(context.Context, string) (*SyncState, error) {
	if s.state == nil {
		return nil, nil
	}
	copyState := *s.state
	return &copyState, nil
}
func (s *memoryStore) SaveState(_ context.Context, state *SyncState) error {
	if s.stateError != nil {
		return s.stateError
	}
	copyState := *state
	s.state = &copyState
	return nil
}
func (s *memoryStore) SaveMessages(_ context.Context, messages []Message) (int, error) {
	if s.saveError != nil {
		return 0, s.saveError
	}
	if s.messages == nil {
		s.messages = map[MessageID]Message{}
	}
	n := 0
	for _, message := range messages {
		if _, ok := s.messages[message.MessageID]; !ok {
			s.messages[message.MessageID] = message
			n++
		}
	}
	return n, nil
}

type pageSource struct {
	pages   map[MessageID][]int
	failAt  MessageID
	failure error
	calls   []MessageID
}

func (s *pageSource) FetchMessages(_ context.Context, req FetchMessagesRequest) (MessagePage, error) {
	s.calls = append(s.calls, req.Before)
	if s.failure != nil && req.Before == s.failAt {
		return MessagePage{}, s.failure
	}
	page := MessagePage{FetchedAt: time.Now().UTC()}
	for _, id := range s.pages[req.Before] {
		page.Messages = append(page.Messages, json.RawMessage(fmt.Sprintf(`{"gid":1,"id":%d,"content":"message"}`, id)))
	}
	return page, nil
}

func TestSyncBudgetFailureAndReplay(t *testing.T) {
	ctx := context.Background()
	store := &memoryStore{state: &SyncState{GroupID: "1", SyncedThroughMessageID: "5", BackfillBeforeMessageID: "1"}}
	source := &pageSource{pages: map[MessageID][]int{"": {10, 9}, "9": {7, 8}, "7": {5, 6}}}
	archiver := NewArchiver(source, store, noopUserSyncer{})
	result, err := archiver.Sync(ctx, "1", 2)
	require.ErrorIs(t, err, ErrSyncIncomplete)
	require.False(t, result.Complete)
	require.Equal(t, 4, result.MessagesSaved)
	require.Equal(t, MessageID("5"), store.state.SyncedThroughMessageID)
	result, err = archiver.Sync(ctx, "1", 3)
	require.NoError(t, err)
	require.True(t, result.Complete)
	require.Equal(t, 1, result.MessagesSaved)
	require.Equal(t, MessageID("10"), store.state.SyncedThroughMessageID)
	require.Equal(t, []MessageID{"", "9", "", "9", "7"}, source.calls)
	result, err = archiver.Sync(ctx, "1", 3)
	require.NoError(t, err)
	require.Zero(t, result.MessagesSaved)
}

func TestSyncFailuresPreserveCheckpoint(t *testing.T) {
	for _, name := range []string{"network", "empty", "stuck", "save", "checkpoint"} {
		t.Run(name, func(t *testing.T) {
			store := &memoryStore{state: &SyncState{GroupID: "1", SyncedThroughMessageID: "5"}}
			source := &pageSource{pages: map[MessageID][]int{"": {10, 9}, "9": {5, 6}}}
			switch name {
			case "network":
				source.failAt = "9"
				source.failure = errors.New("network")
			case "empty":
				source.pages["9"] = nil
			case "stuck":
				source.pages["9"] = []int{9}
			case "save":
				store.saveError = errors.New("database")
			case "checkpoint":
				store.stateError = errors.New("checkpoint")
			}
			result, err := NewArchiver(source, store, noopUserSyncer{}).Sync(context.Background(), "1", 3)
			require.Error(t, err)
			require.False(t, result.Complete)
			require.Equal(t, MessageID("5"), store.state.SyncedThroughMessageID)
		})
	}
}

func TestInitializeAndBackfill(t *testing.T) {
	ctx := context.Background()
	store := &memoryStore{}
	source := &pageSource{pages: map[MessageID][]int{"": {8, 10, 9}, "8": {7, 6}, "6": {4, 5}, "4": {}}}
	archiver := NewArchiver(source, store, noopUserSyncer{})
	_, err := archiver.Sync(ctx, "1", 1)
	require.ErrorIs(t, err, ErrNotInitialized)
	init, err := archiver.Initialize(ctx, "1")
	require.NoError(t, err)
	require.Equal(t, MessageID("10"), init.Through)
	require.Equal(t, MessageID("8"), store.state.InitialMessageID)
	_, err = archiver.Initialize(ctx, "1")
	require.ErrorIs(t, err, ErrAlreadyInitialized)
	backfill, err := archiver.Backfill(ctx, "1", 1)
	require.NoError(t, err)
	require.False(t, backfill.Exhausted)
	require.Equal(t, MessageID("6"), backfill.Before)
	source.failAt = "4"
	source.failure = errors.New("network")
	_, err = archiver.Backfill(ctx, "1", 3)
	require.Error(t, err)
	require.Equal(t, MessageID("4"), store.state.BackfillBeforeMessageID)
	source.failure = nil
	backfill, err = archiver.Backfill(ctx, "1", 1)
	require.NoError(t, err)
	require.True(t, backfill.Exhausted)
	require.Equal(t, MessageID("10"), store.state.SyncedThroughMessageID)
}

func TestEmptyInitializationRequiresReadingToEnd(t *testing.T) {
	ctx := context.Background()
	store := &memoryStore{}
	source := &pageSource{pages: map[MessageID][]int{}}
	archiver := NewArchiver(source, store, noopUserSyncer{})
	_, err := archiver.Initialize(ctx, "1")
	require.NoError(t, err)
	require.NotNil(t, store.state.HistoryExhaustedAt)
	source.pages[""] = []int{2, 1}
	_, err = archiver.Sync(ctx, "1", 1)
	require.ErrorIs(t, err, ErrSyncIncomplete)
	require.Empty(t, store.state.SyncedThroughMessageID)
	result, err := archiver.Sync(ctx, "1", 2)
	require.NoError(t, err)
	require.Equal(t, MessageID("2"), result.Through)
}

// Message pagination tests do not perform profile IO.
type noopUserSyncer struct{}

func (noopUserSyncer) SyncUsers(context.Context, []Message) error { return nil }
