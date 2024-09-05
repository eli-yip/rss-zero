package migrate

import (
	"net/http"
	"time"

	"github.com/rs/xid"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/file"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	zhihuRequest "github.com/eli-yip/rss-zero/pkg/routers/zhihu/request"
)

// Get objects from db, save them to minio
func MigrateMinio20240905(file file.File, db *gorm.DB, logger *zap.Logger) {
	logger = logger.With(zap.String("migrate_id", xid.New().String()))
	logger.Info("Start to migrate minio files")

	zhihuDBService := zhihuDB.NewDBService(db)
	objects, err := zhihuDBService.GetAllObjects()
	if err != nil {
		logger.Error("Failed to get all objects from db", zap.Error(err))
		return
	}

	for _, obj := range objects {
		exist, err := file.Exist(obj.ObjectKey)
		if err != nil {
			logger.Error("Failed to check file existance", zap.Error(err), zap.String("object_key", obj.ObjectKey))
			return
		}

		if exist {
			logger.Info("File already exists", zap.String("object_key", obj.ObjectKey))
			continue
		}

		resp, err := zhihuRequest.NoLimitStream(http.DefaultClient, obj.URL, 3, logger)
		if err != nil {
			logger.Error("Failed to get image stream", zap.Error(err), zap.String("url", obj.URL))
			return
		}

		if err = file.SaveStream(obj.ObjectKey, resp.Body, resp.ContentLength); err != nil {
			logger.Error("Failed to save image stream to file service", zap.Error(err), zap.String("object_key", obj.ObjectKey))
			return
		}

		logger.Info("Save image stream to file service", zap.String("object_key", obj.ObjectKey))

		time.Sleep(5 * time.Second)
	}

	logger.Info("Finish to migrate minio files")
}
