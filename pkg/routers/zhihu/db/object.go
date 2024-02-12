package db

import "github.com/lib/pq"

type Object struct {
	ID              int            `gorm:"column:id;type:text;primary_key"` // Use hash to convert zhihu content url to id
	Type            int            `gorm:"column:type;type:int"`
	ContentType     int            `gorm:"column:content_type;type:int"`
	ContentID       int            `gorm:"column:content_id;type:int"`
	ObjectKey       string         `gorm:"column:object_key;type:text"`
	URL             string         `gorm:"column:url;type:text"`
	StorageProvider pq.StringArray `gorm:"column:storage_provider;type:text[]"`
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
}

func (d *DBService) SaveObjectInfo(o *Object) error {
	return d.Save(o).Error
}

func (d *DBService) GetObjectInfo(id int) (o *Object, err error) {
	o = &Object{}
	err = d.Where("id = ?", id).First(o).Error
	return
}
