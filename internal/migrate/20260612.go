package migrate

import (
	"encoding/json"
	"time"

	"github.com/rs/xid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Migrate20260612 backfills the update_at column for zhihu answers, articles and
// pins that were written before the column existed (the 2024-11-09 AutoMigrate
// left those rows NULL). For each NULL row it takes the greater of the timestamp
// recorded in the stored raw payload and the row's create_at, so the freshness
// rule in the parser no longer needs to special-case a zero update_at.
//
// Once this has run in production and `SELECT count(*) WHERE update_at IS NULL`
// is 0 for all three tables, the IsZero branch in parse.storedIsCurrent can be
// removed.
func Migrate20260612(db *gorm.DB, logger *zap.Logger) {
	logger = logger.With(zap.String("migrate_id", xid.New().String()))
	defer func() {
		if r := recover(); r != nil {
			logger.Error("Panic", zap.Any("recover", r))
		}
	}()

	logger.Info("Start migrate 20260612: backfill zhihu update_at")

	for _, table := range []string{"zhihu_answer", "zhihu_article", "zhihu_pin"} {
		backfillUpdateAt(db, logger, table)
	}

	logger.Info("Migrate 20260612 done")
}

// backfillRow is the minimal projection needed to recover an update_at.
type backfillRow struct {
	ID       int       `gorm:"column:id"`
	Raw      []byte    `gorm:"column:raw"`
	CreateAt time.Time `gorm:"column:create_at"`
}

func backfillUpdateAt(db *gorm.DB, logger *zap.Logger, table string) {
	logger = logger.With(zap.String("table", table))

	var rows []backfillRow
	if err := db.Table(table).
		Select("id, raw, create_at").
		Where("update_at IS NULL").
		Find(&rows).Error; err != nil {
		logger.Error("Failed to load rows to backfill", zap.Error(err))
		return
	}
	logger.Info("Loaded rows to backfill", zap.Int("count", len(rows)))

	var fromRaw, fromCreateAt, failed int
	for _, r := range rows {
		updated := r.CreateAt.Unix()
		source := "create_at"
		if u := rawUpdatedUnix(r.Raw); u > updated {
			updated = u
			source = "raw"
		}

		if err := db.Table(table).
			Where("id = ?", r.ID).
			Update("update_at", time.Unix(updated, 0)).Error; err != nil {
			logger.Error("Failed to update row", zap.Int("id", r.ID), zap.Error(err))
			failed++
			continue
		}

		if source == "raw" {
			fromRaw++
		} else {
			fromCreateAt++
		}
	}

	logger.Info("Backfill table done",
		zap.Int("from_raw", fromRaw),
		zap.Int("from_create_at", fromCreateAt),
		zap.Int("failed", failed))
}

// rawUpdatedUnix extracts the edit timestamp from a stored raw payload, returning
// 0 when absent or unparseable. Answers carry it as "updated_time", articles and
// pins as "updated"; a given payload only sets one, so taking the max is safe.
func rawUpdatedUnix(raw []byte) int64 {
	if len(raw) == 0 {
		return 0
	}
	var p struct {
		UpdatedTime int64 `json:"updated_time"`
		Updated     int64 `json:"updated"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return 0
	}
	if p.Updated > p.UpdatedTime {
		return p.Updated
	}
	return p.UpdatedTime
}
