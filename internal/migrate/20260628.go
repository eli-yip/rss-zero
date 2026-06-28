package migrate

import (
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/pkg/routers/tombkeeper"
)

func init() {
	Register(Migration{
		Version:              20260628000000,
		Name:                 "tombkeeper-zero-byte-image-redownload",
		Auto:                 true,
		RequiresPredecessors: false,
		Run:                  migrateTombkeeperZeroByteImages,
	})
}

// migrateTombkeeperZeroByteImages repairs tombkeeper images stored as 0-byte OSS
// objects. The earlier bug: a third-party image proxy (image.baidu.com/search/down)
// could answer a candidate probe with "200 OK, Content-Length: 0", which won the
// download race, was streamed to OSS as an empty file, and was recorded
// ObjectStatusOK — so the post body embeds a working OSS URL that serves 0 bytes
// (a broken image). GetPicStream now rejects empty responses, but already-stored
// rows are status OK and are never re-fetched by the crawler, so this one-off
// backfill re-downloads them in place (same object key, so the markdown links
// already embedded in post bodies keep working). It is idempotent: a non-empty
// object is skipped, so it is safe to re-run. Returns an error if any 0-byte object
// could not be repaired, so the registry retries it on the next startup rather than
// recording a partial backfill.
func migrateTombkeeperZeroByteImages(db *gorm.DB, logger *zap.Logger) error {
	f, err := file.NewFileServiceMinio(config.C.Minio, logger)
	if err != nil {
		return fmt.Errorf("init minio: %w", err)
	}
	req := tombkeeper.NewRequestService(logger)
	defer req.Close()

	var objs []tombkeeper.Object
	if err := db.Where("type = ? AND status = ?", tombkeeper.ObjectTypeImage, tombkeeper.ObjectStatusOK).
		Find(&objs).Error; err != nil {
		return fmt.Errorf("scan tombkeeper_object: %w", err)
	}

	var scanned, repaired, failed int
	for _, o := range objs {
		if o.ObjectKey == "" {
			continue
		}
		scanned++
		size, err := f.Size(o.ObjectKey)
		if err != nil {
			logger.Warn("failed to stat object, skipping",
				zap.String("object_key", o.ObjectKey), zap.Error(err))
			failed++
			continue
		}
		if size != 0 {
			continue
		}
		usedURL, err := tombkeeper.RedownloadObject(req, f, o.ObjectKey, o.ID, logger)
		if err != nil {
			logger.Error("failed to redownload zero-byte image",
				zap.String("pic_id", o.ID), zap.String("object_key", o.ObjectKey), zap.Error(err))
			failed++
			continue
		}
		// Keep the recorded source URL honest: it was the dead empty-proxy URL.
		if err := db.Model(&tombkeeper.Object{}).Where("id = ?", o.ID).
			Update("url", usedURL).Error; err != nil {
			logger.Warn("repaired bytes but failed to update url",
				zap.String("pic_id", o.ID), zap.Error(err))
		}
		repaired++
	}
	logger.Info("tombkeeper zero-byte image backfill done",
		zap.Int("scanned", scanned), zap.Int("repaired", repaired), zap.Int("failed", failed))
	if failed > 0 {
		return fmt.Errorf("redownload: %d objects failed", failed)
	}
	return nil
}
