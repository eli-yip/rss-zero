package weibochat

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ReadState stores one inclusive read cursor per reader and group.
// A nil cursor means every archived message is unread.
type ReadState struct {
	OwnerUsername        string    `gorm:"type:text;primaryKey"`
	GroupID              string    `gorm:"type:text;primaryKey"`
	ReadThroughMessageID *string   `gorm:"type:text"`
	Revision             int64     `gorm:"not null;default:0"`
	UpdatedAt            time.Time `gorm:"type:timestamptz;not null"`
}

func (ReadState) TableName() string { return "weibo_chat_read_state" }

type ReadStateSnapshot struct {
	ReadThroughMessageID *string `json:"read_through_message_id"`
	FirstUnreadMessageID *string `json:"first_unread_message_id"`
	LatestMessageID      *string `json:"latest_message_id"`
	Revision             int64   `json:"revision"`
}

type ReadStatePatch struct {
	Action    string
	MessageID *string
	Revision  int64
}

var (
	ErrReadStateConflict = errors.New("read state changed; reload before retrying")
	ErrInvalidReadState  = errors.New("invalid read state update")
)

func (s *PostgresStore) GetReadState(ctx context.Context, owner, groupID string) (ReadStateSnapshot, error) {
	var result ReadStateSnapshot
	err := s.db.WithContext(ctx).Raw(`
 SELECT r.read_through_message_id, COALESCE(r.revision, 0) AS revision,
 (SELECT message_id FROM weibo_chat_message m WHERE m.group_id = ?
  ORDER BY length(message_id) DESC, message_id COLLATE "C" DESC LIMIT 1) AS latest_message_id,
 (SELECT message_id FROM weibo_chat_message m WHERE m.group_id = ?
  AND (r.read_through_message_id IS NULL OR
       (length(m.message_id), m.message_id COLLATE "C") >
       (length(r.read_through_message_id), r.read_through_message_id COLLATE "C"))
  ORDER BY length(message_id), message_id COLLATE "C" LIMIT 1) AS first_unread_message_id
 FROM (SELECT 1) seed LEFT JOIN weibo_chat_read_state r ON r.owner_username = ? AND r.group_id = ?`, groupID, groupID, owner, groupID).Scan(&result).Error
	return result, err
}

func (s *PostgresStore) UpdateReadState(ctx context.Context, owner, groupID string, patch ReadStatePatch) (ReadStateSnapshot, error) {
	var result ReadStateSnapshot
	if owner == "" || !MessageID(groupID).Valid() || patch.Revision < 0 {
		return result, ErrInvalidReadState
	}
	switch patch.Action {
	case "advance", "mark_unread", "mark_all_read":
		if patch.MessageID == nil {
			return result, ErrInvalidReadState
		}
	case "restore":
	default:
		return result, ErrInvalidReadState
	}
	if patch.MessageID != nil && !MessageID(*patch.MessageID).Valid() {
		return result, ErrInvalidReadState
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Insert before locking so concurrent first writes serialize on the same row.
		state := ReadState{OwnerUsername: owner, GroupID: groupID}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&state).Error; err != nil {
			return err
		}
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("owner_username = ? AND group_id = ?", owner, groupID).First(&state).Error; err != nil {
			return err
		}
		if state.Revision != patch.Revision {
			return ErrReadStateConflict
		}
		cursor := patch.MessageID
		if cursor != nil {
			var count int64
			if err := tx.Model(&Message{}).Where("group_id = ? AND message_id = ?", groupID, *cursor).Count(&count).Error; err != nil {
				return err
			}
			if count == 0 {
				return ErrMessageNotFound
			}
		}
		if patch.Action == "mark_unread" {
			var previous []string
			if err := tx.Model(&Message{}).Where("group_id = ?", groupID).
				Where(`(length(message_id), message_id COLLATE "C") < (?, ? COLLATE "C")`, len(*cursor), *cursor).
				Order(`length(message_id) DESC, message_id COLLATE "C" DESC`).Limit(1).Pluck("message_id", &previous).Error; err != nil {
				return err
			}
			cursor = nil
			if len(previous) > 0 {
				cursor = &previous[0]
			}
		}
		if (patch.Action == "advance" || patch.Action == "mark_all_read") && state.ReadThroughMessageID != nil && MessageID(*state.ReadThroughMessageID).Compare(MessageID(*cursor)) > 0 {
			cursor = state.ReadThroughMessageID
		}
		if err := tx.Model(&state).Updates(map[string]any{"read_through_message_id": cursor, "revision": state.Revision + 1}).Error; err != nil {
			return err
		}
		var err error
		result, err = NewPostgresStore(tx).GetReadState(ctx, owner, groupID)
		return err
	})
	return result, err
}

// GroupsWithUnread checks all listed groups in one query without counting messages.
func (s *PostgresStore) GroupsWithUnread(ctx context.Context, owner string, groupIDs []string) (map[string]bool, error) {
	result := make(map[string]bool)
	if len(groupIDs) == 0 {
		return result, nil
	}
	if owner == "" {
		return nil, fmt.Errorf("owner is required")
	}
	var rows []struct {
		GroupID   string
		HasUnread bool
	}
	err := s.db.WithContext(ctx).Raw(`SELECT g.group_id, EXISTS (
  SELECT 1 FROM weibo_chat_message m WHERE m.group_id = g.group_id
  AND (r.read_through_message_id IS NULL OR
   (length(m.message_id), m.message_id COLLATE "C") >
   (length(r.read_through_message_id), r.read_through_message_id COLLATE "C"))
 ) AS has_unread FROM (SELECT DISTINCT group_id FROM weibo_chat_message WHERE group_id IN ?) g
 LEFT JOIN weibo_chat_read_state r ON r.owner_username = ? AND r.group_id = g.group_id`, groupIDs, owner).Scan(&rows).Error
	for _, row := range rows {
		result[row.GroupID] = row.HasUnread
	}
	return result, err
}
