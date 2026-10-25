package weibochat

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SyncState records API coverage, not proof of complete historical availability.
type SyncState struct {
	GroupID                 string     `gorm:"type:text;primaryKey"`
	InitializedAt           time.Time  `gorm:"type:timestamptz;not null"`
	InitialMessageID        MessageID  `gorm:"type:text"`
	SyncedThroughMessageID  MessageID  `gorm:"type:text"`
	BackfillBeforeMessageID MessageID  `gorm:"type:text"`
	HistoryExhaustedAt      *time.Time `gorm:"type:timestamptz"`
	LastSyncedAt            *time.Time `gorm:"type:timestamptz"`
}

func (SyncState) TableName() string { return "weibo_chat_sync_state" }

const (
	AssetPending = "pending"
	AssetStored  = "stored"
	AssetFailed  = "failed"
)

type Asset struct {
	GroupID         string `gorm:"type:text;primaryKey"`
	AssetKey        string `gorm:"type:text;primaryKey"`
	Kind            string `gorm:"type:text;not null"`
	SourceURL       string `gorm:"type:text"`
	Status          string `gorm:"type:text;not null"`
	ObjectKey       string `gorm:"type:text"`
	StorageProvider string `gorm:"type:text"`
	ContentType     string `gorm:"type:text"`
	ByteSize        int64
	Width           int
	Height          int
	StoredAt        *time.Time `gorm:"type:timestamptz"`
	LastError       string     `gorm:"type:text"`
}

func (Asset) TableName() string { return "weibo_chat_asset" }

// PostgresStore uses idempotent writes. Callers serialize operations per group.
type PostgresStore struct{ db *gorm.DB }

func NewPostgresStore(db *gorm.DB) *PostgresStore { return &PostgresStore{db: db} }

// SaveMessages keeps the first observation and repairs missing asset rows on replay.
// A failed call may have saved some rows; the caller must not advance its cursor.
func (s *PostgresStore) SaveMessages(ctx context.Context, messages []Message) (int, error) {
	saved := 0
	for _, message := range messages {
		result := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&message)
		if result.Error != nil {
			return saved, fmt.Errorf("save message: %w", result.Error)
		}
		saved += int(result.RowsAffected)
		if result.RowsAffected == 0 {
			if err := s.db.WithContext(ctx).Where("group_id = ? AND message_id = ?", message.GroupID, message.MessageID).First(&message).Error; err != nil {
				return saved, err
			}
		}
		for _, ref := range message.Content.Assets {
			asset := Asset{GroupID: message.GroupID, AssetKey: ref.AssetKey, Kind: ref.Kind, SourceURL: ref.SourceURL, Status: AssetPending}
			if err := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&asset).Error; err != nil {
				return saved, fmt.Errorf("save asset reference: %w", err)
			}
		}
	}
	return saved, nil
}

// GetState returns nil when the group has not been initialized.
func (s *PostgresStore) GetState(ctx context.Context, groupID string) (*SyncState, error) {
	var state SyncState
	err := s.db.WithContext(ctx).Where("group_id = ?", groupID).First(&state).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func (s *PostgresStore) SaveState(ctx context.Context, state *SyncState) error {
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(state).Error
}

// ListAssets returns unfinished assets in stable key order. The caller supplies
// the last visited key so failures are attempted only once in a single run.
func (s *PostgresStore) ListAssets(ctx context.Context, groupID, after string, limit int) ([]Asset, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("asset limit must be positive")
	}
	var assets []Asset
	err := s.db.WithContext(ctx).Where("group_id = ? AND asset_key > ? AND status <> ?", groupID, after, AssetStored).
		Order("asset_key").Limit(limit).Find(&assets).Error
	return assets, err
}

// SaveAssetResult updates an existing asset only after its transfer attempt.
func (s *PostgresStore) SaveAssetResult(ctx context.Context, asset *Asset) error {
	result := s.db.WithContext(ctx).Model(&Asset{}).Where("group_id = ? AND asset_key = ?", asset.GroupID, asset.AssetKey).
		Updates(map[string]any{"source_url": asset.SourceURL, "status": asset.Status, "object_key": asset.ObjectKey, "storage_provider": asset.StorageProvider,
			"content_type": asset.ContentType, "byte_size": asset.ByteSize, "width": asset.Width, "height": asset.Height, "stored_at": asset.StoredAt, "last_error": asset.LastError})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("asset does not exist")
	}
	return nil
}

// ListMessages reads a bounded page without fetching upstream or loading Raw.
func (s *PostgresStore) ListMessages(ctx context.Context, groupID string, filter MessageFilter) (MessageListPage, error) {
	page := MessageListPage{Messages: []Message{}}
	if filter.Limit < 1 || filter.Limit > 100 {
		return page, fmt.Errorf("message limit must be between 1 and 100")
	}
	if filter.BeforeMessageID != "" && filter.AfterMessageID != "" {
		return page, fmt.Errorf("before_message_id and after_message_id are mutually exclusive")
	}
	query := s.db.WithContext(ctx).Model(&Message{}).Omit("Raw").Where("group_id = ?", groupID)
	if filter.StarredOnly {
		query = query.Where("EXISTS (SELECT 1 FROM weibo_chat_user_preference p WHERE p.owner_username = ? AND p.uid = weibo_chat_message.sender_id AND p.is_starred AND NOT p.is_blocked)", filter.OwnerUsername)
	} else if filter.OwnerUsername != "" && len(filter.Search.Terms) > 0 {
		query = query.Where("NOT EXISTS (SELECT 1 FROM weibo_chat_user_preference p WHERE p.owner_username = ? AND p.uid = weibo_chat_message.sender_id AND p.is_blocked)", filter.OwnerUsername)
	}
	for _, term := range filter.Search.Terms {
		query = query.Where(`translate(text, 'ABCDEFGHIJKLMNOPQRSTUVWXYZ', 'abcdefghijklmnopqrstuvwxyz') COLLATE "C" LIKE ? ESCAPE '!'`, searchPattern(term))
	}
	if len(filter.SenderIDs) > 0 {
		query = query.Where("sender_id IN ?", filter.SenderIDs)
	}
	if filter.SentFrom != nil {
		query = query.Where("sent_at >= ?", *filter.SentFrom)
	}
	if filter.SentBefore != nil {
		query = query.Where("sent_at < ?", *filter.SentBefore)
	}
	if len(filter.AssetKinds) > 0 {
		query = query.Where("EXISTS (SELECT 1 FROM jsonb_array_elements(COALESCE(content->'assets', '[]'::jsonb)) AS asset WHERE asset->>'kind' IN ?)", filter.AssetKinds)
	}
	if filter.BeforeMessageID != "" {
		query = query.Where(`(length(message_id), message_id COLLATE "C") < (?, ? COLLATE "C")`, len(filter.BeforeMessageID), string(filter.BeforeMessageID))
	}
	order := `length(message_id) DESC, message_id COLLATE "C" DESC`
	if filter.AfterMessageID != "" {
		query = query.Where(`(length(message_id), message_id COLLATE "C") > (?, ? COLLATE "C")`, len(filter.AfterMessageID), string(filter.AfterMessageID))
		order = `length(message_id) ASC, message_id COLLATE "C" ASC`
	}
	if err := query.Order(order).Limit(filter.Limit + 1).Find(&page.Messages).Error; err != nil {
		return page, err
	}
	if len(page.Messages) > filter.Limit {
		page.Messages = page.Messages[:filter.Limit]
		page.HasMore = true
	}
	if filter.AfterMessageID == "" {
		slices.Reverse(page.Messages)
	}
	return page, nil
}

// Group describes observed archive coverage. A group with messages but no saved
// sync state remains discoverable after an interrupted initialization.
type Group struct {
	GroupID       string
	InitializedAt *time.Time
	LastSyncedAt  *time.Time
}

func (s *PostgresStore) ListGroups(ctx context.Context) ([]Group, error) {
	groups := []Group{}
	err := s.db.WithContext(ctx).Raw(`
		SELECT groups.group_id, state.initialized_at, state.last_synced_at
		FROM (SELECT group_id FROM weibo_chat_message UNION SELECT group_id FROM weibo_chat_sync_state) AS groups
		LEFT JOIN weibo_chat_sync_state AS state USING (group_id)
		ORDER BY length(groups.group_id), groups.group_id COLLATE "C"
	`).Scan(&groups).Error
	return groups, err
}

// GetAssets returns records for a page's references, including pending and failed
// assets. Callers retain reference order when assembling individual messages.
func (s *PostgresStore) GetAssets(ctx context.Context, groupID string, keys []string) ([]Asset, error) {
	assets := []Asset{}
	if len(keys) == 0 {
		return assets, nil
	}
	err := s.db.WithContext(ctx).Where("group_id = ? AND asset_key IN ?", groupID, keys).Find(&assets).Error
	return assets, err
}

var ErrMessageNotFound = errors.New("archived message not found")

// GetMessageContext does not apply search or sender filters. Both sides are
// selected through the same paging rules as the message list.
func (s *PostgresStore) GetMessageContext(ctx context.Context, groupID string, messageID MessageID, beforeLimit, afterLimit int) (MessageContext, error) {
	result := MessageContext{AnchorMessageID: messageID, Messages: []Message{}}
	if beforeLimit < 0 || beforeLimit > 50 || afterLimit < 0 || afterLimit > 50 {
		return result, fmt.Errorf("context limits must be between 0 and 50")
	}
	var anchor Message
	err := s.db.WithContext(ctx).Omit("Raw").Where("group_id = ? AND message_id = ?", groupID, messageID).First(&anchor).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return result, ErrMessageNotFound
	}
	if err != nil {
		return result, err
	}
	before, err := s.ListMessages(ctx, groupID, MessageFilter{BeforeMessageID: messageID, Limit: max(1, beforeLimit)})
	if err != nil {
		return result, err
	}
	after, err := s.ListMessages(ctx, groupID, MessageFilter{AfterMessageID: messageID, Limit: max(1, afterLimit)})
	if err != nil {
		return result, err
	}
	result.HasMoreBefore, result.HasMoreAfter = before.HasMore, after.HasMore
	if beforeLimit == 0 {
		result.HasMoreBefore = len(before.Messages) > 0
	} else {
		result.Messages = append(result.Messages, before.Messages...)
	}
	result.Messages = append(result.Messages, anchor)
	if afterLimit == 0 {
		result.HasMoreAfter = len(after.Messages) > 0
	} else {
		result.Messages = append(result.Messages, after.Messages...)
	}
	return result, nil
}
