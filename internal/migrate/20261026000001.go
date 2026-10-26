package migrate

import (
	"encoding/json"
	"fmt"

	"github.com/eli-yip/rss-zero/internal/weibochat"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func init() {
	Register(Migration{Version: 20261026000001, Name: "weibo-chat-file-references", Auto: true, Run: migrateWeiboFileReferences})
}

// migrateWeiboFileReferences repairs derived references only. Downloads remain
// the asset archiver's responsibility and retry independently after startup.
func migrateWeiboFileReferences(db *gorm.DB, logger *zap.Logger) error {
	group, id := "", ""
	updated := int64(0)
	for {
		var messages []weibochat.Message
		if err := db.Select("group_id", "message_id", "raw").
			Where("media_type = ?", 5).
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
				return fmt.Errorf("parse archived file message %s/%s: %w", message.GroupID, message.MessageID, err)
			}
			if parsed.ParseError != "" {
				return fmt.Errorf("parse archived file message %s/%s: %s", message.GroupID, message.MessageID, parsed.ParseError)
			}
			for _, ref := range parsed.Content.Assets {
				if ref.Kind != weibochat.AssetFile {
					continue
				}
				encoded, err := json.Marshal([]weibochat.AssetRef{ref})
				if err != nil {
					return err
				}
				// SQL patches preserve all unknown content fields and existing
				// references. A retry also repairs an absent asset row.
				result := db.Model(&weibochat.Message{}).
					Where("group_id = ? AND message_id = ?", message.GroupID, message.MessageID).
					Where("NOT EXISTS (SELECT 1 FROM jsonb_array_elements(COALESCE(content->'assets', '[]'::jsonb)) AS ref WHERE ref->>'asset_key' = ?)", ref.AssetKey).
					Update("content", gorm.Expr("jsonb_set(content, '{assets}', COALESCE(content->'assets', '[]'::jsonb) || ?::jsonb, true)", string(encoded)))
				if result.Error != nil {
					return result.Error
				}
				updated += result.RowsAffected
				asset := weibochat.Asset{GroupID: message.GroupID, AssetKey: ref.AssetKey, Kind: ref.Kind, SourceURL: ref.SourceURL, Status: weibochat.AssetPending}
				if err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&asset).Error; err != nil {
					return err
				}
			}
		}
		last := messages[len(messages)-1]
		group, id = last.GroupID, string(last.MessageID)
	}
	logger.Info("weibo file references backfilled", zap.Int64("updated", updated))
	return nil
}
