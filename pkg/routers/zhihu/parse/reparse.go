package parse

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// loadOrAbsent fetches a record by id, normalizing gorm.ErrRecordNotFound into a
// nil record with no error so callers can treat "absent" uniformly. It lets the
// three zhihu parsers share one existence check instead of one helper each.
func loadOrAbsent[T any](get func(int) (*T, error), id int) (*T, error) {
	rec, err := get(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return rec, err
}

// storedIsCurrent reports whether an already-stored record still reflects the
// incoming payload, i.e. it does NOT need re-parsing. stored is the timestamp in
// the database; incoming is the updated_at carried by the freshly fetched
// content.
//
// A zero stored timestamp marks a row written before zhihu content carried an
// updated_at (pre-2024-11-09 AutoMigrate left those rows NULL). Its freshness is
// unknown, so we conservatively treat it as current and skip re-parsing. Once the
// 20260612 backfill has filled those rows in production this branch is dead and
// can be removed, collapsing the rule to `return !incoming.After(stored)`.
func storedIsCurrent(stored, incoming time.Time) bool {
	if stored.IsZero() {
		return true
	}
	return !incoming.After(stored)
}
