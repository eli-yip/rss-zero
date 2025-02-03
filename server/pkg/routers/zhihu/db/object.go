package db

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

type Object struct {
	ID              int            `gorm:"column:id;type:text;primary_key"` // Use hash to convert zhihu content url to id
	Type            int            `gorm:"column:type;type:int"`
	ContentType     int            `gorm:"column:content_type;type:int"`
	ContentID       int            `gorm:"column:content_id;type:int"`
	ObjectKey       string         `gorm:"column:object_key;type:text"`
	URL             string         `gorm:"column:url;type:text"`
	StorageProvider pq.StringArray `gorm:"column:storage_provider;type:text[]"`
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

func (d *DBService) GetAllObjects() (objects []*Object, err error) {
	var objs []*Object
	err = d.Find(&objs).Error
	return objs, err
}
