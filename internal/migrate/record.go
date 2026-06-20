package migrate

import (
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"gorm.io/gorm"
)

// SchemaMigration records one applied registry migration. Its presence (by
// Version) marks that migration complete; the set of rows is the source of
// truth for what has run. Created via AutoMigrate (see MigrateDB).
type SchemaMigration struct {
	Version   int64     `gorm:"column:version;primaryKey"`
	Name      string    `gorm:"column:name;type:text"`
	AppliedAt time.Time `gorm:"column:applied_at;type:timestamptz"`
}

func (SchemaMigration) TableName() string { return "schema_migrations" }

// loadApplied returns the set of completed migration versions.
func loadApplied(db *gorm.DB) (mapset.Set[int64], error) {
	var versions []int64
	if err := db.Model(&SchemaMigration{}).Pluck("version", &versions).Error; err != nil {
		return nil, err
	}
	return mapset.NewSet(versions...), nil
}

// recordApplied marks a migration version complete.
func recordApplied(db *gorm.DB, version int64, name string) error {
	return db.Create(&SchemaMigration{Version: version, Name: name, AppliedAt: time.Now()}).Error
}
