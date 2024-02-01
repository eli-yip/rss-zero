package db

import (
	"github.com/lib/pq"
)

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
	ObjectImageType = iota
)

const (
	ContentTypeAnswer = iota
)

//	"author": {
//	  "name": "墨苍离",
//	  "url_token": "canglimo"
//	}
type Author struct {
	ID   string `gorm:"column:id;type:text;primary_key"` // url_token in zhihu api resp
	Name string `gorm:"column:name;type:text"`
}

func (a *Author) TableName() string { return "zhihu_author" }

type Sub struct {
	AuthorID string `gorm:"column:author_id;type:text;primary_key"`
	Type     int    `gorm:"column:type;type:int;primary_key"`
}

func (s *Sub) TableName() string { return "zhihu_sub" }
