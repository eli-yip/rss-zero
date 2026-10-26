package migrate

import (
	"encoding/json"
	"fmt"

	"github.com/eli-yip/rss-zero/internal/weibochat"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func init() {
	Register(Migration{Version: 20261026000000, Name: "weibo-chat-group-events", Auto: true, Run: migrateWeiboGroupEvents})
}

func migrateWeiboGroupEvents(db *gorm.DB, logger *zap.Logger) error {
	if err := db.AutoMigrate(&weibochat.Message{}); err != nil {
		return err
	}
	group, id := "", ""
	updated := int64(0)
	for {
		var messages []weibochat.Message
		if err := db.Select("group_id", "message_id", "raw").
			Where("event_type = '' AND message_type IN ?", []int{100, 321, 322, 344}).
			Where("(group_id, message_id) > (?, ?)", group, id).
			Order("group_id, message_id").Limit(100).Find(&messages).Error; err != nil {
			return err
		}
		if len(messages) == 0 {
			break
		}
		for _, message := range messages {
			parsed, err := weibochat.ParseMessage(message.Raw)
			if err != nil {
				return fmt.Errorf("parse archived message %s/%s: %w", message.GroupID, message.MessageID, err)
			}
			if parsed.EventType == weibochat.EventNone {
				continue
			}
			actor, err := json.Marshal(parsed.Content.EventActor)
			if err != nil {
				return err
			}
			// Patch only event metadata; existing content may contain fields unknown
			// to this binary. Keep text, links, assets and raw observations intact.
			result := db.Model(&weibochat.Message{}).
				Where("group_id = ? AND message_id = ? AND event_type = ''", message.GroupID, message.MessageID).
				Updates(map[string]any{
					"event_type": parsed.EventType,
					"sender_id":  "",
					"content":    gorm.Expr("jsonb_set(content, '{event_actor}', ?::jsonb, true)", string(actor)),
				})
			if result.Error != nil {
				return result.Error
			}
			updated += result.RowsAffected
		}
		last := messages[len(messages)-1]
		group, id = last.GroupID, string(last.MessageID)
	}
	logger.Info("weibo group events backfilled", zap.Int64("updated", updated))
	return nil
}
