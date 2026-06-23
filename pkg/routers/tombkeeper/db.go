package tombkeeper

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/file"
)

// Post is a tombkeeper weibo stored for the feed. The numeric weibo id (mid) is
// the canonical primary key; bid is kept only for back-links/debugging.
type Post struct {
	ID           int64          `gorm:"column:id;type:bigint;primary_key"`
	Bid          string         `gorm:"column:bid;type:text"`
	AuthorID     string         `gorm:"column:author_id;type:text"`
	ScreenName   string         `gorm:"column:screen_name;type:text"`
	PostTime     time.Time      `gorm:"column:created_at;type:timestamptz;index"`
	Title        string         `gorm:"column:title;type:text"`
	TextMarkdown string         `gorm:"column:text_markdown;type:text"`
	VideoURL     string         `gorm:"column:video_url;type:text"`
	RetweetID    string         `gorm:"column:retweet_id;type:text"`
	Raw          []byte         `gorm:"column:raw;type:bytea"`
	CreatedDBAt  time.Time      `gorm:"column:created_db_at;autoCreateTime"`
	UpdatedDBAt  time.Time      `gorm:"column:updated_db_at;autoUpdateTime"`
	DeletedAt    gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (*Post) TableName() string { return "tombkeeper_post" }

const ObjectTypeImage = 0

// ObjectStatus values: 0 = stored to OSS, 1 = abandoned (all CDNs unreachable;
// the post body keeps the original image link instead).
const (
	ObjectStatusOK        = 0
	ObjectStatusAbandoned = 1
)

// Object is a rehosted (or abandoned) media item belonging to a post. id is the
// bare sinaimg pic id.
type Object struct {
	ID              string         `gorm:"column:id;type:text;primary_key"`
	PostID          int64          `gorm:"column:post_id;type:bigint"`
	Type            int            `gorm:"column:type;type:int"`
	ObjectKey       string         `gorm:"column:object_key;type:text"`
	URL             string         `gorm:"column:url;type:text"`
	StorageProvider pq.StringArray `gorm:"column:storage_provider;type:text[]"`
	Status          int            `gorm:"column:status;type:int"`
	CreatedAt       time.Time      `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt       time.Time      `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt       gorm.DeletedAt `gorm:"column:deleted_at"`
}

func (*Object) TableName() string { return "tombkeeper_object" }

// URI builds the public OSS URL for a stored object.
func (o *Object) URI() (string, error) {
	return file.ObjectURI(o.StorageProvider, o.ObjectKey)
}

type DBPost interface {
	SavePost(p *Post) error
	GetPost(id int64) (*Post, error)
	PostExists(id int64) (bool, error)
	GetLatestPosts(n int) ([]Post, error)
}

type DBObject interface {
	SaveObject(o *Object) error
	GetObject(id string) (*Object, error)
	ObjectExists(id string) (bool, error)
}

type DB interface {
	DBPost
	DBObject
}

type DBService struct{ *gorm.DB }

func NewDBService(db *gorm.DB) DB { return &DBService{db} }

func (d *DBService) SavePost(p *Post) error { return d.Save(p).Error }

func (d *DBService) GetPost(id int64) (*Post, error) {
	p := &Post{}
	err := d.Where("id = ?", id).First(p).Error
	return p, err
}

func (d *DBService) PostExists(id int64) (bool, error) {
	var count int64
	if err := d.Model(&Post{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

func (d *DBService) GetLatestPosts(n int) ([]Post, error) {
	var posts []Post
	err := d.Order("created_at DESC").Limit(n).Find(&posts).Error
	return posts, err
}

func (d *DBService) SaveObject(o *Object) error { return d.Save(o).Error }

func (d *DBService) GetObject(id string) (*Object, error) {
	o := &Object{}
	err := d.Where("id = ?", id).First(o).Error
	return o, err
}

func (d *DBService) ObjectExists(id string) (bool, error) {
	var count int64
	if err := d.Model(&Object{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
