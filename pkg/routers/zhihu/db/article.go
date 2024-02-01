package db

import "time"

type Article struct {
	ID       int       `gorm:"column:id;type:int;primary_key"`
	AuthorID string    `gorm:"column:author_id;type:text"`
	CreateAt time.Time `gorm:"column:create_at;type:timestamp"`
	Title    string    `gorm:"column:title;type:text"`
	Text     string    `gorm:"column:text;type:text"`
	Raw      []byte    `gorm:"column:raw;type:bytea"`
}

func (p *Article) TableName() string { return "zhihu_article" }

type DataBasePost interface {
	SaveArticle(p *Article) error
}

func (d *DBService) SaveArticle(p *Article) error {
	return d.Save(p).Error
}
