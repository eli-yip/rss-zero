package db

import (
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

type Object struct {
	ID              int                     `gorm:"column:id;type:text;primary_key"` // Use hash to convert zhihu content url to id
	Type            int                     `gorm:"column:type;type:int"`
	ContentType     common.ZhihuContentType `gorm:"column:content_type;type:int"`
	ContentID       int                     `gorm:"column:content_id;type:int"`
	ObjectKey       string                  `gorm:"column:object_key;type:text"`
	URL             string                  `gorm:"column:url;type:text"`
	StorageProvider pq.StringArray          `gorm:"column:storage_provider;type:text[]"`
	// Note: some older records don't have created_at and updated_at columns
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt gorm.DeletedAt
}

func (o *Object) TableName() string { return "zhihu_object" }

const (
	ObjectTypeImage = iota
)

type DBObject interface {
	// Save object info to zhihu_object table
	SaveObjectInfo(o *Object) error
	// Get object info from zhihu_object table
	GetObjectInfo(oid int) (o *Object, err error)
	// GetObjectsByIDs 按 id 批量读取对象事实，供读取期 ContentLoader 一次装配快照、避免逐图 N+1。
	GetObjectsByIDs(ids []int) ([]Object, error)
	// Get all objects from zhihu_object table
	GetAllObjects() (objects []*Object, err error)
}

func (d *DBService) SaveObjectInfo(o *Object) error {
	return d.Save(o).Error
}

func (d *DBService) GetObjectInfo(id int) (o *Object, err error) {
	o = &Object{}
	err = d.Where("id = ?", id).First(o).Error
	return
}

// GetObjectsByIDs 与 GetObjectInfo 同样按（哈希得来的）int id 查 text 主键，只是一次取多条。
func (d *DBService) GetObjectsByIDs(ids []int) ([]Object, error) {
	objects := make([]Object, 0, len(ids))
	if err := d.Where("id in ?", ids).Find(&objects).Error; err != nil {
		return nil, fmt.Errorf("failed to get objects by ids: %w", err)
	}
	return objects, nil
}

func (d *DBService) GetAllObjects() (objects []*Object, err error) {
	var objs []*Object
	err = d.Find(&objs).Error
	return objs, err
}
