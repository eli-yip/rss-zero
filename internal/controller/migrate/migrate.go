package controller

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/controller/common"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/migrate"
	"github.com/eli-yip/rss-zero/pkg/httputil"
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
		return httputil.NewHTTPError(http.StatusInternalServerError, "Failed to create minio service")
	}

	logger.Info("Start to migrate minio files")

	go migrate.MigrateMinio20240905(minioService, h.db, logger)

	return c.JSON(http.StatusOK, httputil.NewMessage("Start to migrate minio files"))
}

func (h *Controller) Migrate20240929(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	logger.Info("Start to migrate db 20240929")

	go migrate.MigrateDB20240929(h.db, logger)

	return c.JSON(http.StatusOK, httputil.NewMessage("Start to migrate db 20240929"))
}

func (h *Controller) Migrate20250530(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	logger.Info("Start to migrate db 20250530")

	go migrate.Migrate20250530(h.db, logger)

	return c.JSON(http.StatusOK, httputil.NewMessage("Start to migrate db 20250530"))
}

func (h *Controller) Migrate20260612(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	logger.Info("Start to migrate db 20260612")

	go migrate.Migrate20260612(h.db, logger)

	return c.JSON(http.StatusOK, httputil.NewMessage("Start to migrate db 20260612"))
}

// MigrationRegistry returns the status of every registry-managed migration.
func (h *Controller) MigrationRegistry(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	statuses, err := migrate.Status(h.db)
	if err != nil {
		logger.Error("Failed to get migration status", zap.Error(err))
		return httputil.NewHTTPError(http.StatusInternalServerError, "Failed to get migration status")
	}

	return c.JSON(http.StatusOK, httputil.NewResp("ok", statuses))
}

// RunMigration manually runs a single registry migration by version,
// synchronously, so eligibility errors and the result are returned to the caller.
func (h *Controller) RunMigration(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	version, err := strconv.ParseInt(c.Param("version"), 10, 64)
	if err != nil {
		return httputil.NewHTTPError(http.StatusBadRequest, "Invalid migration version")
	}

	if err = migrate.RunVersion(h.db, logger, version); err != nil {
		logger.Error("Failed to run migration", zap.Int64("version", version), zap.Error(err))
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	return c.JSON(http.StatusOK, httputil.NewMessage(fmt.Sprintf("Migration %d applied", version)))
}

// RunPendingMigrations runs all eligible registry migrations (including non-auto).
func (h *Controller) RunPendingMigrations(c echo.Context) (err error) {
	logger := common.ExtractLogger(c)

	logger.Info("Start to run pending migrations")

	go migrate.RunPending(h.db, logger)

	return c.JSON(http.StatusOK, httputil.NewMessage("Start to run pending migrations"))
}
