package weibochat

import (
	"context"
	"errors"
	"fmt"
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
		Updates(map[string]any{"status": asset.Status, "object_key": asset.ObjectKey, "storage_provider": asset.StorageProvider,
			"content_type": asset.ContentType, "byte_size": asset.ByteSize, "stored_at": asset.StoredAt, "last_error": asset.LastError})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("asset does not exist")
	}
	return nil
}
