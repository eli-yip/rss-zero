package tombkeeper

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/eli-yip/rss-zero/internal/file"
)

// Post 是已归档博文的结构化内容；展示字段均在读取时推导。
type Post struct {
	ID              int64               `gorm:"column:id;type:bigint;primaryKey"`
	Bid             string              `gorm:"column:bid;type:text"`
	AuthorID        string              `gorm:"column:author_id;type:text"`
	ScreenName      string              `gorm:"column:screen_name;type:text"`
	PublishedAt     time.Time           `gorm:"column:published_at;type:timestamptz;index"`
	Text            string              `gorm:"column:text;type:text"`
	Pics            pq.StringArray      `gorm:"column:pics;type:text[]"`
	Links           []PostLink          `gorm:"column:url_info;type:jsonb;serializer:json"`
	H5ImageIDsByURL map[string][]string `gorm:"column:view_pics;type:jsonb;serializer:json"`
	RetweetPostID   int64               `gorm:"column:retweet_post_id;type:bigint"`
	InTimeline      bool                `gorm:"column:in_timeline;type:boolean;not null;default:false;index"`
	CreatedDBAt     time.Time           `gorm:"column:created_db_at;autoCreateTime"`
	UpdatedDBAt     time.Time           `gorm:"column:updated_db_at;autoUpdateTime"`
	DeletedAt       gorm.DeletedAt      `gorm:"column:deleted_at"`
}

func (*Post) TableName() string { return "tombkeeper_post" }

const ObjectTypeImage = 0

const (
	ObjectStatusOK        = 0
	ObjectStatusAbandoned = 1
)

// ImageAsset 是源图片及其 OSS 存储结果，沿用旧表以复用已有资产。
type ImageAsset struct {
	ID              string         `gorm:"column:id;type:text;primaryKey"`
	Type            int            `gorm:"column:type;type:int"`
	ObjectKey       string         `gorm:"column:object_key;type:text"`
	URL             string         `gorm:"column:url;type:text"`
	StorageProvider pq.StringArray `gorm:"column:storage_provider;type:text[]"`
	Status          int            `gorm:"column:status;type:int"`
	CreatedAt       time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time      `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (*ImageAsset) TableName() string { return "tombkeeper_object" }

func (a *ImageAsset) URI() (string, error) {
	return file.ObjectURI(a.StorageProvider, a.ObjectKey)
}

type ContentReader interface {
	GetPost(id int64) (*Post, error)
	GetPosts(ids []int64) ([]Post, error)
	GetImageAssets(ids []string) ([]ImageAsset, error)
}

type ImportStore interface {
	GetPosts(ids []int64) ([]Post, error)
	UpsertPost(post *Post) error
	SaveImageAsset(asset *ImageAsset) error
	ImageAssetExists(id string) (bool, error)
}

type DB interface {
	ImportStore
	ContentReader
	LatestTimelineEntries(n int) ([]Post, error)
}

type DBService struct{ *gorm.DB }

func NewDBService(db *gorm.DB) DB { return &DBService{db} }

// UpsertPost 刷新源内容，同时保证时间线成员与 H5 图片索引不会降级。
func (d *DBService) UpsertPost(post *Post) error {
	if post.H5ImageIDsByURL == nil {
		post.H5ImageIDsByURL = map[string][]string{}
	}
	return d.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"bid":             clause.Expr{SQL: "EXCLUDED.bid"},
			"author_id":       clause.Expr{SQL: "EXCLUDED.author_id"},
			"screen_name":     clause.Expr{SQL: "EXCLUDED.screen_name"},
			"published_at":    clause.Expr{SQL: "EXCLUDED.published_at"},
			"text":            clause.Expr{SQL: "EXCLUDED.text"},
			"pics":            clause.Expr{SQL: "EXCLUDED.pics"},
			"url_info":        clause.Expr{SQL: "EXCLUDED.url_info"},
			"retweet_post_id": clause.Expr{SQL: "EXCLUDED.retweet_post_id"},
			"in_timeline":     clause.Expr{SQL: "tombkeeper_post.in_timeline OR EXCLUDED.in_timeline"},
			"view_pics":       clause.Expr{SQL: mergeH5ImageIDsSQL},
			"updated_db_at":   clause.Expr{SQL: "CURRENT_TIMESTAMP"},
		}),
	}).Create(post).Error
}

const mergeH5ImageIDsSQL = `(
	SELECT COALESCE(jsonb_object_agg(k, CASE
		WHEN jsonb_array_length(COALESCE(EXCLUDED.view_pics, '{}'::jsonb) -> k) > 0
			THEN EXCLUDED.view_pics -> k
		WHEN COALESCE(tombkeeper_post.view_pics, '{}'::jsonb) ? k
			THEN tombkeeper_post.view_pics -> k
		ELSE EXCLUDED.view_pics -> k
	END), '{}'::jsonb)
	FROM jsonb_object_keys(
		COALESCE(tombkeeper_post.view_pics, '{}'::jsonb) ||
		COALESCE(EXCLUDED.view_pics, '{}'::jsonb)
	) AS keys(k)
)`

func (d *DBService) GetPost(id int64) (*Post, error) {
	post := &Post{}
	err := d.Where("id = ?", id).First(post).Error
	return post, err
}

func (d *DBService) GetPosts(ids []int64) ([]Post, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var posts []Post
	err := d.Where("id IN ?", ids).Find(&posts).Error
	return posts, err
}

func (d *DBService) LatestTimelineEntries(n int) ([]Post, error) {
	var posts []Post
	err := d.Where("in_timeline = ?", true).Order("published_at DESC").Limit(n).Find(&posts).Error
	return posts, err
}

func (d *DBService) SaveImageAsset(asset *ImageAsset) error {
	return d.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"type": clause.Expr{SQL: "EXCLUDED.type"},
			"object_key": clause.Expr{SQL: `CASE WHEN tombkeeper_object.status = 0
				THEN tombkeeper_object.object_key ELSE EXCLUDED.object_key END`},
			"url": clause.Expr{SQL: `CASE WHEN tombkeeper_object.status = 0
				THEN tombkeeper_object.url ELSE EXCLUDED.url END`},
			"storage_provider": clause.Expr{SQL: `CASE WHEN tombkeeper_object.status = 0
				THEN tombkeeper_object.storage_provider ELSE EXCLUDED.storage_provider END`},
			"status": clause.Expr{SQL: "LEAST(tombkeeper_object.status, EXCLUDED.status)"},
			"updated_at": clause.Expr{SQL: "CURRENT_TIMESTAMP"},
		}),
	}).Create(asset).Error
}

func (d *DBService) GetImageAsset(id string) (*ImageAsset, error) {
	asset := &ImageAsset{}
	err := d.Where("id = ?", id).First(asset).Error
	return asset, err
}

func (d *DBService) GetImageAssets(ids []string) ([]ImageAsset, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var assets []ImageAsset
	err := d.Where("id IN ?", ids).Find(&assets).Error
	return assets, err
}

func (d *DBService) ImageAssetExists(id string) (bool, error) {
	var count int64
	if err := d.Model(&ImageAsset{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
