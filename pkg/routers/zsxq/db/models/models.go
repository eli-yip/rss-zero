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
	AuthorID  int       `gorm:"column:author_id"`
	ShareLink string    `gorm:"column:share_link;type:text"`
	Title     *string   `gorm:"column:title;type:text"`
	Text      string    `gorm:"column:text;type:text"`
	Raw       []byte    `gorm:"column:raw;type:bytea"`
}

func (t *Topic) TableName() string { return "zsxq_topic" }

type Article struct {
	ID    string `gorm:"column:id;primary_key"`
	URL   string `gorm:"column:url;type:text"`
	Title string `gorm:"column:title;type:text"`
	Text  string `gorm:"column:text;type:text"`
	Raw   []byte `gorm:"column:raw;type:bytea"`
}

func (a *Article) TableName() string { return "zsxq_article" }

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

type Group struct {
	ID         int       `gorm:"column:id;primary_key"`
	Name       string    `gorm:"column:name;type:text"`
	UpdateAt   time.Time `gorm:"column:update_at"`
	ErrorTimes int       `gorm:"column:error_times;type:int"`
}

func (g *Group) TableName() string { return "zsxq_group" }

type Author struct {
	ID    int     `gorm:"column:id;primary_key"`
	Name  string  `gorm:"column:name;type:text"`
	Alias *string `gorm:"column:alias;type:text"`
}

func (a *Author) TableName() string { return "zsxq_author" }
