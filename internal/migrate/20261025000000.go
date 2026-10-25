package migrate

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/weibochat"
)

func init() {
	Register(Migration{Version: 20261025000000, Name: "weibo-chat-image-dimensions", Auto: true, Run: migrateWeiboImageDimensions})
}

func migrateWeiboImageDimensions(db *gorm.DB, logger *zap.Logger) error {
	var files file.ContextFile
	return backfillWeiboImageDimensions(db, logger, func(ctx context.Context, asset weibochat.Asset) (io.ReadCloser, error) {
		if files == nil {
			service, err := file.NewFileServiceMinio(config.C.Minio, logger)
			if err != nil {
				return nil, err
			}
			var ok bool
			files, ok = service.(file.ContextFile)
			if !ok {
				return nil, fmt.Errorf("storage does not support request deadlines")
			}
		}
		if strings.TrimRight(asset.StorageProvider, "/") != strings.TrimRight(files.AssetsDomain(), "/") {
			return nil, fmt.Errorf("asset uses a different storage provider")
		}
		return files.GetStreamContext(ctx, asset.ObjectKey)
	})
}

func backfillWeiboImageDimensions(db *gorm.DB, logger *zap.Logger, open func(context.Context, weibochat.Asset) (io.ReadCloser, error)) error {
	// Also support running this version directly through the existing migration API.
	if err := db.AutoMigrate(&weibochat.Asset{}); err != nil {
		return err
	}
	group, key := "", ""
	failed, updated := 0, 0
	for {
		var assets []weibochat.Asset
		err := db.Where("status = ? AND kind IN ? AND (COALESCE(width, 0) <= 0 OR COALESCE(height, 0) <= 0)", weibochat.AssetStored, []string{"image", "sticker"}).
			Where("(group_id, asset_key) > (?, ?)", group, key).
			Order("group_id, asset_key").Limit(100).Find(&assets).Error
		if err != nil {
			return err
		}
		if len(assets) == 0 {
			break
		}
		for _, asset := range assets {
			err := backfillWeiboImageDimension(db, asset, open)
			if err != nil {
				failed++
				logger.Error("weibo image dimensions failed", zap.String("group_id", asset.GroupID), zap.String("asset_key", asset.AssetKey), zap.Error(err))
			} else {
				updated++
			}
		}
		last := assets[len(assets)-1]
		group, key = last.GroupID, last.AssetKey
	}
	logger.Info("weibo image dimensions backfilled", zap.Int("updated", updated), zap.Int("failed", failed))
	if failed > 0 {
		return fmt.Errorf("image dimensions: %d assets failed", failed)
	}
	return nil
}

func backfillWeiboImageDimension(db *gorm.DB, asset weibochat.Asset, open func(context.Context, weibochat.Asset) (io.ReadCloser, error)) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	body, err := open(ctx, asset)
	if err != nil {
		return err
	}
	defer body.Close()
	width, height, err := weibochat.ReadImageDimensions(body)
	if err != nil {
		return err
	}
	return db.WithContext(ctx).Model(&weibochat.Asset{}).
		Where("group_id = ? AND asset_key = ? AND object_key = ?", asset.GroupID, asset.AssetKey, asset.ObjectKey).
		Updates(map[string]any{"width": width, "height": height}).Error
}
