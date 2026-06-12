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
// content. A record is current unless the incoming payload was edited after it.
//
// A zero stored timestamp (which the 20260612 migration backfilled away) now
// sorts as the oldest possible time, so such a row would be re-parsed once.
func storedIsCurrent(stored, incoming time.Time) bool {
	return !incoming.After(stored)
}
