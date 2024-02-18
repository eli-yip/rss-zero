package db

import (
	"time"

	"github.com/lib/pq"
)

type Object struct {
	ID              int            `gorm:"column:id;primary_key"`
	TopicID         int            `gorm:"column:topic_id"`
	Time            time.Time      `gorm:"column:time"`
	Type            string         `gorm:"column:type;type:text"`
	ObjectKey       string         `gorm:"column:object_key;type:text"`
	StorageProvider pq.StringArray `gorm:"column:storage_provider;type:text[]"`
	Transcript      string         `gorm:"column:transcript;type:text"`
}

func (o *Object) TableName() string { return "zsxq_object" }

type DBObject interface {
	// Save object info to zsxq_object table
	SaveObjectInfo(o *Object) error
	// Get object info from zsxq_object table
	GetObjectInfo(oid int) (o *Object, err error)
}

func (s *ZsxqDBService) SaveObjectInfo(o *Object) error {
	return s.db.Save(o).Error
}

func (s *ZsxqDBService) GetObjectInfo(oid int) (*Object, error) {
	var object Object
	if err := s.db.Where("id = ?", oid).First(&object).Error; err != nil {
		return nil, err
	}
	return &object, nil
}
