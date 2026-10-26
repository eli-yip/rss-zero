package migrate

import (
	"github.com/eli-yip/rss-zero/internal/weibochat"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func init() {
	Register(Migration{Version: 20261026000002, Name: "weibo-chat-asset-display", Auto: true, Run: migrateWeiboAssetDisplay})
}

func migrateWeiboAssetDisplay(db *gorm.DB, _ *zap.Logger) error {
	// Failed HEIC assets are retried by the normal archiver after startup.
	return db.AutoMigrate(&weibochat.Asset{})
}
