package db

import (
	"time"

	"gorm.io/gorm"
)

type Article struct {
	ID       int       `gorm:"column:id;type:int;primary_key"`
	AuthorID string    `gorm:"column:author_id;type:text"`
	CreateAt time.Time `gorm:"column:create_at;type:timestamp with time zone"`
	Title    string    `gorm:"column:title;type:text"`
	Text     string    `gorm:"column:text;type:text"`
	Raw      []byte    `gorm:"column:raw;type:bytea"`
}

func (p *Article) TableName() string { return "zhihu_article" }

type DataBasePost interface {
	SaveArticle(p *Article) error
	GetLatestArticleTime(authorID string) (time.Time, error)
}

func (d *DBService) SaveArticle(p *Article) error {
	return d.Save(p).Error
}

func (d *DBService) GetLatestArticleTime(userID string) (time.Time, error) {
	var t time.Time
	if err := d.Model(&Article{}).Where("author_id = ?", userID).Order("create_at desc").Limit(1).Pluck("create_at", &t).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return t, nil
}
