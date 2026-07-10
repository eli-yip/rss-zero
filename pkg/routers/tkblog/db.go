package tkblog

import (
	"time"

	"gorm.io/gorm"
)

// Post is one tombkeeper blog article (xfocus or baidu). The site-generated id is
// an opaque token (e.g. "26ho1i8FcjS"), unique only within a category, so the
// primary key is the (category, id) pair — mirroring the composite-PK precedent in
// github/db (PK (gh_user, name)).
type Post struct {
	Category     string         `gorm:"primaryKey;column:category;type:text"`
	ID           string         `gorm:"primaryKey;column:id;type:text"`
	Title        string         `gorm:"column:title;type:text"`
	CreatedAt    time.Time      `gorm:"column:created_at;type:timestamptz;index"`
	TextMarkdown string         `gorm:"column:text_markdown;type:text"`
	SourceURL    string         `gorm:"column:source_url;type:text"` // Wayback Machine original link
	CreatedDBAt  time.Time      `gorm:"column:created_db_at;autoCreateTime"`
	UpdatedDBAt  time.Time      `gorm:"column:updated_db_at;autoUpdateTime"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (*Post) TableName() string { return "tombkeeper_blog_post" }

type DB interface {
	SavePost(p *Post) error
	GetPost(category, id string) (*Post, error)
}

type DBService struct{ *gorm.DB }

func NewDBService(db *gorm.DB) DB { return &DBService{db} }

// SavePost upserts by primary key: GORM v2 Save UPDATEs when the (category, id) PK
// is set and CREATEs otherwise, so a re-crawl of the same article is idempotent —
// no explicit clause.OnConflict needed (same as github/db SaveRepo).
func (d *DBService) SavePost(p *Post) error { return d.Save(p).Error }

func (d *DBService) GetPost(category, id string) (*Post, error) {
	p := &Post{}
	err := d.Where("category = ? AND id = ?", category, id).First(p).Error
	return p, err
}
