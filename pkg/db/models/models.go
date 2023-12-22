package models

import (
	"time"

	"github.com/lib/pq"
)

type Topic struct {
	ID        int       `gorm:"column:id;primary_key"`
	Time      time.Time `gorm:"column:time"`
	Type      string    `gorm:"column:type;type:text"`
	Digested  bool      `gorm:"column:digested;type:bool"`
	Author    string    `gorm:"column:author;type:text"`
	ShareLink string    `gorm:"column:share_link;type:text"`
	Text      string    `gorm:"column:text;type:text"`
	Raw       []byte    `gorm:"column:raw;type:bytea"`
}

type Object struct {
	ID              int            `gorm:"column:id;primary_key"`
	TopicID         int            `gorm:"column:topic_id"`
	Time            time.Time      `gorm:"column:time"`
	Type            string         `gorm:"column:type;type:text"`
	StorageProvider pq.StringArray `gorm:"column:storage_provider;type:text[]"`
	Transcript      string         `gorm:"column:transcript;type:text"`
}
