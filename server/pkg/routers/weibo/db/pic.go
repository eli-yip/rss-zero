package db

type Object struct {
	ID              string `gorm:"column:id;type:text;primary_key"`
	Type            int    `gorm:"column:type;type:int"`
	ContentID       int    `gorm:"column:content_id;type:int"`
	ObjectKey       string `gorm:"column:object_key;type:text"`
	URL             string `gorm:"column:url;type:text"`
	StorageProvider string `gorm:"column:storage_provider;type:text"`
}

func (o *Object) TableName() string { return "weibo_object" }

const ObjectTypeImage = iota

type DBObject interface {
	SaveObjectInfo(o *Object) (err error)
	GetObjectInfo(id string) (o *Object, err error)
}

func (d *DBService) SaveObjectInfo(o *Object) (err error) { return d.Save(o).Error }

func (d *DBService) GetObjectInfo(id string) (o *Object, err error) {
	o = &Object{}
	err = d.Where("id = ?", id).First(o).Error
	return o, err
}
