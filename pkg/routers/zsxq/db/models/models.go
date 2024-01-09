// This file defines the models in database.
package models

import (
	"time"

	"github.com/lib/pq"
)

type Topic struct {
	ID        int       `gorm:"column:id;primary_key"`
	Time      time.Time `gorm:"column:time"`
	GroupID   int       `gorm:"column:group_id"`
	Type      string    `gorm:"column:type;type:text"`
	Digested  bool      `gorm:"column:digested;type:bool"`
	Author    string    `gorm:"column:author;type:text"`
	ShareLink string    `gorm:"column:share_link;type:text"`
	Title     string    `gorm:"column:title;type:text"`
	Text      string    `gorm:"column:text;type:text"`
	Raw       []byte    `gorm:"column:raw;type:bytea"`
}

func (t *Topic) TableName() string { return "zsxq_topics" }

type Object struct {
	ID              int            `gorm:"column:id;primary_key"`
	TopicID         int            `gorm:"column:topic_id"`
	Time            time.Time      `gorm:"column:time"`
	Type            string         `gorm:"column:type;type:text"`
	ObjectKey       string         `gorm:"column:object_key;type:text"`
	StorageProvider pq.StringArray `gorm:"column:storage_provider;type:text[]"`
	Transcript      string         `gorm:"column:transcript;type:text"`
}

func (o *Object) TableName() string { return "zsxq_objects" }

type Group struct {
	ID         int       `gorm:"column:id;primary_key"`
	Name       string    `gorm:"column:name;type:text"`
	UpdateAt   time.Time `gorm:"column:update_at"`
	ErrorTimes int       `gorm:"column:error_times;type:int"`
}

func (g *Group) TableName() string { return "zsxq_groups" }
