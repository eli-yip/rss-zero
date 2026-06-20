package migrate

import (
	"fmt"
	"sort"

	mapset "github.com/deckarep/golang-set/v2"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Migration is one registry-managed, forward-only data migration.
//
// Version is an int64 timestamp formatted YYYYMMDDHHMMSS and must be unique
// across the registry. The registry is APPEND-ONLY: never delete or renumber a
// published/applied migration — validateRegistry rejects duplicates and the
// out-of-order detection relies on versions only ever growing.
//
// Completeness does not depend on contiguous versions: on every startup the set
// difference (registry − applied) is run in ascending order, so anything not yet
// applied is eventually applied regardless of gaps.
type Migration struct {
	Version              int64
	Name                 string
	Auto                 bool // run automatically on startup
	RequiresPredecessors bool // only eligible once every smaller version is applied
	Run                  func(db *gorm.DB, logger *zap.Logger) error
}

// registry holds the process-wide migrations, populated by Register (typically
// from a migration file's init()).
var registry []Migration

// Register adds a migration to the package registry.
func Register(m Migration) { registry = append(registry, m) }

// validateRegistry rejects non-positive or duplicate versions.
func validateRegistry(ms []Migration) error {
	seen := make(map[int64]string, len(ms))
	for _, m := range ms {
		if m.Version <= 0 {
			return fmt.Errorf("migration %q has non-positive version %d", m.Name, m.Version)
		}
		if other, dup := seen[m.Version]; dup {
			return fmt.Errorf("duplicate migration version %d (%q and %q)", m.Version, other, m.Name)
		}
		seen[m.Version] = m.Name
	}
	return nil
}

// predecessorsDone reports whether every registered migration with a smaller
// version is already applied.
func predecessorsDone(m Migration, all []Migration, applied mapset.Set[int64]) bool {
	for _, o := range all {
		if o.Version < m.Version && !applied.Contains(o.Version) {
			return false
		}
	}
	return true
}

// outOfOrder reports whether an unapplied version is older than the newest
// already-applied one — i.e. it was introduced after a later version ran.
func outOfOrder(version, priorMax int64) bool { return priorMax > 0 && version < priorMax }

// sortedByVersion returns a copy of all sorted ascending by version.
func sortedByVersion(all []Migration) []Migration {
	out := make([]Migration, len(all))
	copy(out, all)
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out
}

// maxVersion returns the largest version in applied, or 0 when empty.
func maxVersion(applied mapset.Set[int64]) int64 {
	var hi int64
	for v := range applied.Iter() {
		if v > hi {
			hi = v
		}
	}
	return hi
}
