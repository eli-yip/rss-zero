package db

import (
	"time"

	"github.com/lib/pq"
	"gorm.io/gorm"
)

type Object struct {
	ID              int            `gorm:"column:id;primary_key"`
	TopicID         int            `gorm:"column:topic_id"`
	Time            time.Time      `gorm:"column:time"`
	Type            string         `gorm:"column:type;type:text"`
	ObjectKey       string         `gorm:"column:object_key;type:text"`
	StorageProvider pq.StringArray `gorm:"column:storage_provider;type:text[]"`
	Transcript      string         `gorm:"column:transcript;type:text"`
	// Note: for zsxq files, download link maybe expired, not testes yet.
	// If it's expired, we can get another download link by requesting api with file id
	Url string `gorm:"column:url;type:text"`
	// Note: some older records don't have created_at and updated_at columns
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
	DeletedAt gorm.DeletedAt
}

func (o *Object) TableName() string { return "zsxq_object" }

type DBObject interface {
	// Save object info to zsxq_object table
	SaveObjectInfo(o *Object) error
	// Get object info from zsxq_object table
	GetObjectInfo(oid int) (o *Object, err error)
}

func (s *ZsxqDBService) SaveObjectInfo(o *Object) error { return s.db.Save(o).Error }

func (s *ZsxqDBService) GetObjectInfo(oid int) (*Object, error) {
	var object Object
	if err := s.db.Where("id = ?", oid).First(&object).Error; err != nil {
		return nil, err
	}
	return &object, nil
}
