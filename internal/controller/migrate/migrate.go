package controller

import (
	"net/http"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/migrate"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type Controller struct {
	logger *zap.Logger
	db     *gorm.DB
}

func NewController(logger *zap.Logger, db *gorm.DB) *Controller {
	return &Controller{
		logger: logger,
		db:     db,
	}
}

func (h *Controller) Migrate20240905(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	minioService, err := file.NewFileServiceMinio(config.C.Minio, logger)
	if err != nil {
		logger.Error("Failed to create minio service", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, "Failed to create minio service")
	}

	logger.Info("Start to migrate minio files")

	go migrate.MigrateMinio20240905(minioService, h.db, logger)

	return c.JSON(http.StatusOK, "Start to migrate minio files")
}

func (h *Controller) Migrate20240929(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	logger.Info("Start to migrate db 20240929")

	go migrate.MigrateDB20240929(h.db, logger)

	return c.JSON(http.StatusOK, "Start to migrate db 20240929")
}
