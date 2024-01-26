package db

import (
	"time"

	"github.com/lib/pq"
)

//	"question": {
//	  "created": 1705768292,
//	  "id": 640511134,
//	  "title": "为什么那么多人就是不愿意承认女生保守是一个极大的竞争优势？"
//	}
type Question struct {
	ID          int       `gorm:"column:id;type:int;primary_key"`
	CreatedTime time.Time `gorm:"column:created_time;type:timestamp"`
	Title       string    `gorm:"column:title;type:text"`
}

func (q *Question) TableName() string { return "zhihu_question" }

type Object struct {
	ID              int            `gorm:"column:id;type:text;primary_key"` // Use hash to convert zhihu content url to id
	Type            int            `gorm:"column:type;type:int"`
	ContentType     int            `gorm:"column:content_type;type:int"`
	ContentID       int            `gorm:"column:content_id;type:int"`
	ObjectKey       string         `gorm:"column:object_key;type:text"`
	URL             string         `gorm:"column:url;type:text"`
	StorageProvider pq.StringArray `gorm:"column:storage_provider;type:text[]"`
}

const (
	ObjectImageType = iota
)

const (
	ContentTypeAnswer = iota
)

func (o *Object) TableName() string { return "zhihu_object" }

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

type Post struct {
	ID          int       `gorm:"column:id;type:int;primary_key"`
	AuthorID    string    `gorm:"column:author_id;type:text"`
	CreatedTime time.Time `gorm:"column:created_time;type:timestamp"`
	Title       string    `gorm:"column:title;type:text"`
	Text        string    `gorm:"column:text;type:text"`
	Raw         []byte    `gorm:"column:raw;type:bytea"`
}

func (p *Post) TableName() string { return "zhihu_post" }

type Pin struct {
	ID          int       `gorm:"column:id;type:int;primary_key"`
	AuthorID    string    `gorm:"column:author_id;type:string"`
	CreatedTime time.Time `gorm:"column:created_time;type:timestamp"`
	Text        string    `gorm:"column:text;type:text"`
}

func (p *Pin) TableName() string { return "zhihu_pin" }
