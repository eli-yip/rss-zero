package weibochat

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	ErrNotInitialized     = errors.New("group archive is not initialized")
	ErrAlreadyInitialized = errors.New("group archive is already initialized")
	ErrSyncIncomplete     = errors.New("message sync did not reach the previous checkpoint")
)

type MessageSource interface {
	FetchMessages(context.Context, FetchMessagesRequest) (MessagePage, error)
}

type MessageStore interface {
	SaveMessages(context.Context, []Message) (int, error)
	GetState(context.Context, string) (*SyncState, error)
	SaveState(context.Context, *SyncState) error
}

// Archiver is a one-shot orchestration service. All operations for the same group,
// including backfill and asset transfer, must be serialized by the caller.
type Archiver struct {
	source MessageSource
	store  MessageStore
}

func NewArchiver(source MessageSource, store MessageStore) *Archiver {
	return &Archiver{source: source, store: store}
}

type SyncResult struct {
	Pages         int
	MessagesSaved int
	Through       MessageID
	Complete      bool
}

type BackfillResult struct {
	Pages         int
	MessagesSaved int
	Before        MessageID
	Exhausted     bool
}

// Initialize explicitly starts coverage at the latest page (up to 20 messages).
// Use Backfill separately to extend coverage into older history.
func (a *Archiver) Initialize(ctx context.Context, groupID string) (SyncResult, error) {
	var result SyncResult
	if !MessageID(groupID).Valid() {
		return result, fmt.Errorf("invalid group ID")
	}
	state, err := a.store.GetState(ctx, groupID)
	if err != nil {
		return result, err
	}
	if state != nil {
		return result, ErrAlreadyInitialized
	}
	messages, low, high, err := a.fetch(ctx, groupID, "")
	if err != nil {
		return result, err
	}
	result.Pages = 1
	result.MessagesSaved, err = a.store.SaveMessages(ctx, messages)
	if err != nil {
		return result, err
	}
	now := time.Now().UTC()
	state = &SyncState{GroupID: groupID, InitializedAt: now, InitialMessageID: low, SyncedThroughMessageID: high, BackfillBeforeMessageID: low, LastSyncedAt: &now}
	if len(messages) == 0 {
		state.HistoryExhaustedAt = &now
	}
	if err = a.store.SaveState(ctx, state); err != nil {
		return result, err
	}
	result.Through, result.Complete = high, true
	return result, nil
}

// Sync preserves the old checkpoint on any failure, including the page budget.
// Saved messages remain durable and are deduplicated on the next run.
func (a *Archiver) Sync(ctx context.Context, groupID string, maxPages int) (SyncResult, error) {
	var result SyncResult
	if maxPages <= 0 {
		return result, fmt.Errorf("maxPages must be positive")
	}
	state, err := a.state(ctx, groupID)
	if err != nil {
		return result, err
	}
	old := state.SyncedThroughMessageID
	high := old
	var before MessageID
	for range maxPages {
		messages, low, pageHigh, err := a.fetch(ctx, groupID, before)
		if err != nil {
			return result, err
		}
		result.Pages++
		if result.Pages == 1 && pageHigh.Compare(high) > 0 {
			high = pageHigh
		}
		if len(messages) == 0 && old != "" {
			return result, ErrSyncIncomplete
		}
		var fresh []Message
		for _, message := range messages {
			if message.MessageID.Compare(old) > 0 {
				fresh = append(fresh, message)
			}
		}
		n, err := a.store.SaveMessages(ctx, fresh)
		result.MessagesSaved += n
		if err != nil {
			return result, err
		}
		connected := (old == "" && len(messages) == 0) || (old != "" && low.Compare(old) <= 0)
		if connected {
			now := time.Now().UTC()
			state.SyncedThroughMessageID = high
			state.LastSyncedAt = &now
			if err = a.store.SaveState(ctx, state); err != nil {
				return result, err
			}
			result.Through, result.Complete = high, true
			return result, nil
		}
		before = low
	}
	return result, ErrSyncIncomplete
}

// Backfill commits each page independently. Reaching maxPages is a normal pause.
func (a *Archiver) Backfill(ctx context.Context, groupID string, maxPages int) (BackfillResult, error) {
	var result BackfillResult
	if maxPages <= 0 {
		return result, fmt.Errorf("maxPages must be positive")
	}
	state, err := a.state(ctx, groupID)
	if err != nil {
		return result, err
	}
	result.Before = state.BackfillBeforeMessageID
	if state.HistoryExhaustedAt != nil {
		result.Exhausted = true
		return result, nil
	}
	for range maxPages {
		messages, low, _, err := a.fetch(ctx, groupID, state.BackfillBeforeMessageID)
		if err != nil {
			return result, err
		}
		result.Pages++
		n, err := a.store.SaveMessages(ctx, messages)
		result.MessagesSaved += n
		if err != nil {
			return result, err
		}
		if len(messages) == 0 {
			now := time.Now().UTC()
			state.HistoryExhaustedAt = &now
		} else {
			state.BackfillBeforeMessageID = low
		}
		if err = a.store.SaveState(ctx, state); err != nil {
			return result, err
		}
		result.Before = state.BackfillBeforeMessageID
		result.Exhausted = state.HistoryExhaustedAt != nil
		if result.Exhausted {
			return result, nil
		}
	}
	return result, nil
}

func (a *Archiver) state(ctx context.Context, groupID string) (*SyncState, error) {
	if !MessageID(groupID).Valid() {
		return nil, fmt.Errorf("invalid group ID")
	}
	state, err := a.store.GetState(ctx, groupID)
	if err != nil {
		return nil, err
	}
	if state == nil {
		return nil, ErrNotInitialized
	}
	return state, nil
}

func (a *Archiver) fetch(ctx context.Context, groupID string, before MessageID) ([]Message, MessageID, MessageID, error) {
	page, err := a.source.FetchMessages(ctx, FetchMessagesRequest{GroupID: groupID, Before: before})
	if err != nil {
		return nil, "", "", err
	}
	var messages []Message
	var low, high MessageID
	for _, raw := range page.Messages {
		message, err := ParseMessage(raw)
		if err != nil {
			return nil, "", "", err
		}
		if message.GroupID != groupID {
			return nil, "", "", fmt.Errorf("message belongs to another group")
		}
		if before != "" && message.MessageID.Compare(before) > 0 {
			return nil, "", "", fmt.Errorf("page exceeds requested cursor")
		}
		message.FetchedAt = page.FetchedAt
		messages = append(messages, message)
		if low == "" || message.MessageID.Compare(low) < 0 {
			low = message.MessageID
		}
		if message.MessageID.Compare(high) > 0 {
			high = message.MessageID
		}
	}
	if len(messages) > 0 && before != "" && low.Compare(before) >= 0 {
		return nil, "", "", fmt.Errorf("message cursor did not advance")
	}
	return messages, low, high, nil
}
