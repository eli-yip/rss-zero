package migrate

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/pkg/routers/tombkeeper"
)

func init() {
	Register(Migration{
		Version: 20260903000000,
		Name:    "tombkeeper-ipfsscan-image-repair",
		Auto:    true,
		Run:     migrateTombkeeperIPFSImages,
	})
}

const imageRepairBatchSize = 100

type imageRepairStore interface {
	ListAfter(string, int) ([]tombkeeper.ImageAsset, error)
	Update(tombkeeper.ImageAsset, tombkeeper.ImageAsset) error
}

type imageRepairDB struct{ db *gorm.DB }

func (d imageRepairDB) ListAfter(after string, limit int) ([]tombkeeper.ImageAsset, error) {
	var assets []tombkeeper.ImageAsset
	err := d.db.Where("type = ? AND id > ?", tombkeeper.ObjectTypeImage, after).
		Order("id").Limit(limit).Find(&assets).Error
	return assets, err
}

func (d imageRepairDB) Update(old, next tombkeeper.ImageAsset) error {
	var updatedAt any
	if !old.UpdatedAt.IsZero() {
		updatedAt = old.UpdatedAt
	}
	result := d.db.Model(&tombkeeper.ImageAsset{}).
		Where("id = ? AND url = ? AND COALESCE(object_key, '') = ? AND COALESCE(status, 0) = ? AND updated_at IS NOT DISTINCT FROM ?",
			old.ID, old.URL, old.ObjectKey, old.Status, updatedAt).
		Updates(map[string]any{
			"object_key": next.ObjectKey, "url": next.URL,
			"storage_provider": next.StorageProvider, "status": next.Status,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("image %s changed during repair", old.ID)
	}
	return nil
}

func isIPFSImageSource(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") &&
		strings.EqualFold(u.Hostname(), "cdn.ipfsscan.io")
}

type imageRepairStats struct{ Scanned, Healthy, Repaired, Failed int }

func (s imageRepairStats) fields() []zap.Field {
	return []zap.Field{zap.Int("scanned", s.Scanned), zap.Int("healthy", s.Healthy),
		zap.Int("repaired", s.Repaired), zap.Int("failed", s.Failed)}
}

type imageRepairer struct {
	store    imageRepairStore
	files    file.ContextFile
	download func(string) (*http.Response, string, error)
	logger   *zap.Logger
	timeout  time.Duration
}

func migrateTombkeeperIPFSImages(db *gorm.DB, logger *zap.Logger) error {
	store := imageRepairDB{db}
	var worker *imageRepairer
	var requester tombkeeper.Requester
	defer func() {
		if requester != nil {
			requester.Close()
		}
	}()
	_, err := runImageRepair(store, logger, func(asset tombkeeper.ImageAsset) (bool, error) {
		// 首个目标才初始化存储和请求器，空库无需外部服务。
		if worker == nil {
			f, err := file.NewFileServiceMinio(config.C.Minio, logger)
			if err != nil {
				return false, err
			}
			contextFile, ok := f.(file.ContextFile)
			if !ok {
				return false, errors.New("storage does not support request deadlines")
			}
			requester = tombkeeper.NewRequestService(logger)
			worker = &imageRepairer{store: store, files: contextFile, logger: logger, timeout: 30 * time.Second,
				download: func(id string) (*http.Response, string, error) {
					return tombkeeper.DownloadImage(requester, id)
				}}
		}
		return worker.repair(asset)
	})
	return err
}

func runImageRepair(store imageRepairStore, logger *zap.Logger,
	repair func(tombkeeper.ImageAsset) (bool, error),
) (imageRepairStats, error) {
	var stats imageRepairStats
	after := ""
	for {
		batch, err := store.ListAfter(after, imageRepairBatchSize)
		if err != nil {
			return stats, fmt.Errorf("scan image assets: %w", err)
		}
		if len(batch) == 0 {
			break
		}
		for _, asset := range batch {
			if !isIPFSImageSource(asset.URL) {
				continue
			}
			stats.Scanned++
			changed, err := repair(asset)
			if err != nil {
				stats.Failed++
				logger.Error("image repair failed", zap.String("pic_id", asset.ID), zap.Error(err))
			} else if changed {
				stats.Repaired++
			} else {
				stats.Healthy++
			}
		}
		after = batch[len(batch)-1].ID
		logger.Info("image repair progress", stats.fields()...)
	}
	logger.Info("tombkeeper IPFS image repair done", stats.fields()...)
	if stats.Failed > 0 {
		return stats, fmt.Errorf("image repair: %d assets failed", stats.Failed)
	}
	return stats, nil
}

func (r *imageRepairer) check(asset tombkeeper.ImageAsset) error {
	if asset.ObjectKey == "" {
		return tombkeeper.ErrNotImage
	}
	if len(asset.StorageProvider) == 0 ||
		strings.TrimRight(asset.StorageProvider[0], "/") != strings.TrimRight(r.files.AssetsDomain(), "/") {
		return fmt.Errorf("unknown storage provider for %s", asset.ID)
	}
	ctx, cancel := context.WithTimeout(context.Background(), r.timeout)
	defer cancel()
	body, err := r.files.GetStreamContext(ctx, asset.ObjectKey)
	if err != nil {
		return err
	}
	validated, _, err := tombkeeper.ValidateImageStream(body)
	if err != nil {
		return err
	}
	return validated.Close()
}

func (r *imageRepairer) repair(old tombkeeper.ImageAsset) (bool, error) {
	err := r.check(old)
	if err == nil {
		if old.Status == tombkeeper.ObjectStatusOK {
			return false, nil
		}
		next := old
		next.Status = tombkeeper.ObjectStatusOK
		return true, r.store.Update(old, next)
	}
	var storageErr minio.ErrorResponse
	missing := errors.As(err, &storageErr) && storageErr.Code == "NoSuchKey"
	if !errors.Is(err, tombkeeper.ErrNotImage) && !missing {
		return false, fmt.Errorf("inspect stored image: %w", err)
	}
	resp, usedURL, err := r.download(old.ID)
	if err != nil {
		return false, fmt.Errorf("download replacement: %w", err)
	}
	defer resp.Body.Close()
	next := old
	next.ObjectKey = "tombkeeper/repair-20260903/" + old.ID + "." + tombkeeper.ImageExtension(resp.Header.Get("Content-Type"))
	next.URL = usedURL
	next.StorageProvider = []string{r.files.AssetsDomain()}
	next.Status = tombkeeper.ObjectStatusOK
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := r.files.SaveStreamContext(ctx, next.ObjectKey, resp.Body, resp.ContentLength); err != nil {
		return false, fmt.Errorf("save replacement: %w", err)
	}
	if err := r.store.Update(old, next); err != nil {
		return false, fmt.Errorf("update replacement: %w", err)
	}
	r.logger.Info("image repaired", zap.String("pic_id", old.ID), zap.String("source", next.URL),
		zap.String("object_key", next.ObjectKey))
	return true, nil
}
