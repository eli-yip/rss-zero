package migrate

import (
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

const tombkeeperH5ImageIndexConstraint = "tombkeeper_post_view_pics_arrays"

func init() {
	Register(Migration{
		Version:              20260715000000,
		Name:                 "tombkeeper-h5-image-index-invariant",
		Auto:                 true,
		RequiresPredecessors: true,
		Run:                  migrateTombkeeperH5ImageIndexInvariant,
	})
}

// migrateTombkeeperH5ImageIndexInvariant 清理旧 H5 图片索引，并让数据库强制维护 object→array 结构。
func migrateTombkeeperH5ImageIndexInvariant(db *gorm.DB, _ *zap.Logger) error {
	if db.Migrator().HasConstraint("tombkeeper_post", tombkeeperH5ImageIndexConstraint) {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		const normalize = `UPDATE tombkeeper_post
SET view_pics = CASE
	WHEN view_pics IS NULL OR jsonb_typeof(view_pics) <> 'object' THEN '{}'::jsonb
	ELSE (
		SELECT COALESCE(jsonb_object_agg(key, CASE
			WHEN jsonb_typeof(value) = 'array' THEN value
			ELSE '[]'::jsonb
		END), '{}'::jsonb)
		FROM jsonb_each(view_pics)
	)
END
WHERE view_pics IS NULL
	OR jsonb_typeof(view_pics) <> 'object'
	OR jsonb_path_exists(view_pics, 'strict $.* ? (@.type() != "array")')`
		if err := tx.Exec(normalize).Error; err != nil {
			return fmt.Errorf("normalize tombkeeper H5 image indexes: %w", err)
		}
		if err := tx.Exec(`ALTER TABLE tombkeeper_post
			ALTER COLUMN view_pics SET DEFAULT '{}'::jsonb,
			ALTER COLUMN view_pics SET NOT NULL`).Error; err != nil {
			return fmt.Errorf("constrain tombkeeper view_pics column: %w", err)
		}
		const addConstraint = `ALTER TABLE tombkeeper_post
ADD CONSTRAINT tombkeeper_post_view_pics_arrays CHECK (
	jsonb_typeof(view_pics) = 'object'
	AND NOT jsonb_path_exists(view_pics, 'strict $.* ? (@.type() != "array")')
)`
		if err := tx.Exec(addConstraint).Error; err != nil {
			return fmt.Errorf("add tombkeeper H5 image index constraint: %w", err)
		}
		return nil
	})
}
