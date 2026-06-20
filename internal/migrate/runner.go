package migrate

import (
	"fmt"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/notify"
)

// notifyFailure sends a Bark notification for a migration failure. It is a no-op
// when no notifier is wired (e.g. tests), so callers need not guard nil.
func notifyFailure(notifier notify.Notifier, logger *zap.Logger, content string) {
	if notifier == nil {
		return
	}
	notify.NoticeWithLogger(notifier, "Migration failed", content, logger)
}

// RunAuto validates the registry and runs every eligible Auto migration on
// startup, in ascending version order. Failures are logged, never fatal — the
// server still starts (decision: availability over strictness). A malformed
// registry (duplicate/non-positive version) is a programmer error and panics.
//
// Single-instance only: this is called once at startup and dedupes via the
// applied set. Multi-instance deployments would need a pg_advisory_xact_lock
// around the run.
func RunAuto(db *gorm.DB, logger *zap.Logger, notifier notify.Notifier) {
	if err := validateRegistry(registry); err != nil {
		logger.Panic("invalid migration registry", zap.Error(err))
	}
	applied, err := loadApplied(db)
	if err != nil {
		logger.Error("Failed to load applied migrations; skipping auto-migration", zap.Error(err))
		notifyFailure(notifier, logger, fmt.Sprintf("failed to load applied migrations: %v", err))
		return
	}
	runSchedule(registry, applied, true, runWith(db, logger, notifier), logger)
}

// RunPending runs all eligible migrations (including non-Auto), for manual
// catch-up. Failures are logged, not returned.
func RunPending(db *gorm.DB, logger *zap.Logger, notifier notify.Notifier) {
	if err := validateRegistry(registry); err != nil {
		logger.Error("Invalid migration registry", zap.Error(err))
		return
	}
	applied, err := loadApplied(db)
	if err != nil {
		logger.Error("Failed to load applied migrations", zap.Error(err))
		notifyFailure(notifier, logger, fmt.Sprintf("failed to load applied migrations: %v", err))
		return
	}
	runSchedule(registry, applied, false, runWith(db, logger, notifier), logger)
}

// RunVersion manually runs a single migration by version, enforcing the same
// eligibility rules (not already applied; predecessors satisfied when required).
func RunVersion(db *gorm.DB, logger *zap.Logger, version int64, notifier notify.Notifier) error {
	if err := validateRegistry(registry); err != nil {
		return err
	}
	var target *Migration
	for i := range registry {
		if registry[i].Version == version {
			target = &registry[i]
			break
		}
	}
	if target == nil {
		return fmt.Errorf("no migration with version %d", version)
	}
	applied, err := loadApplied(db)
	if err != nil {
		return err
	}
	if applied.Contains(version) {
		return fmt.Errorf("migration %d already applied", version)
	}
	if target.RequiresPredecessors && !predecessorsDone(*target, registry, applied) {
		return fmt.Errorf("migration %d requires all earlier migrations to be applied first", version)
	}
	return runWith(db, logger, notifier)(*target)
}

// runSchedule is the pure scheduling core: it walks migrations in ascending
// version order and runs eligible ones via run, updating applied as each
// succeeds so same-batch predecessors are honored. autoOnly limits to Auto
// migrations. Per-migration errors are logged and skipped, never fatal.
func runSchedule(all []Migration, applied mapset.Set[int64], autoOnly bool,
	run func(Migration) error, logger *zap.Logger,
) {
	priorMax := maxVersion(applied)
	for _, m := range sortedByVersion(all) {
		if applied.Contains(m.Version) {
			continue
		}
		if outOfOrder(m.Version, priorMax) {
			logger.Warn("Migration is out of order (introduced after a newer migration ran)",
				zap.Int64("version", m.Version), zap.String("name", m.Name))
		}
		if autoOnly && !m.Auto {
			continue
		}
		if m.RequiresPredecessors && !predecessorsDone(m, all, applied) {
			logger.Info("Migration waiting for predecessors",
				zap.Int64("version", m.Version), zap.String("name", m.Name))
			continue
		}
		if err := run(m); err != nil {
			logger.Error("Migration failed",
				zap.Int64("version", m.Version), zap.String("name", m.Name), zap.Error(err))
			continue
		}
		applied.Add(m.Version)
	}
}

// runWith returns the side-effecting runner: recover-wrapped Run, recording the
// version only on success.
func runWith(db *gorm.DB, logger *zap.Logger, notifier notify.Notifier) func(Migration) error {
	return func(m Migration) (err error) {
		l := logger.With(zap.Int64("version", m.Version), zap.String("name", m.Name))
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("migration panicked: %v", r)
			}
			if err != nil {
				notifyFailure(notifier, l, fmt.Sprintf("migration %d (%s) failed: %v", m.Version, m.Name, err))
			}
		}()
		l.Info("Running migration")
		if err = m.Run(db, l); err != nil {
			return err
		}
		if err = recordApplied(db, m.Version, m.Name); err != nil {
			return fmt.Errorf("migration ran but recording it failed: %w", err)
		}
		l.Info("Migration applied")
		return nil
	}
}

// MigrationStatus is the read-only view returned to the status endpoint.
type MigrationStatus struct {
	Version              int64      `json:"version"`
	Name                 string     `json:"name"`
	Auto                 bool       `json:"auto"`
	RequiresPredecessors bool       `json:"requires_predecessors"`
	Completed            bool       `json:"completed"`
	AppliedAt            *time.Time `json:"applied_at,omitempty"`
}

// Status reports every registered migration with its completion state.
func Status(db *gorm.DB) ([]MigrationStatus, error) {
	var records []SchemaMigration
	if err := db.Find(&records).Error; err != nil {
		return nil, err
	}
	byVersion := make(map[int64]SchemaMigration, len(records))
	for _, r := range records {
		byVersion[r.Version] = r
	}

	all := sortedByVersion(registry)
	out := make([]MigrationStatus, 0, len(all))
	for _, m := range all {
		st := MigrationStatus{
			Version:              m.Version,
			Name:                 m.Name,
			Auto:                 m.Auto,
			RequiresPredecessors: m.RequiresPredecessors,
		}
		if r, ok := byVersion[m.Version]; ok {
			st.Completed = true
			appliedAt := r.AppliedAt
			st.AppliedAt = &appliedAt
		}
		out = append(out, st)
	}
	return out, nil
}
