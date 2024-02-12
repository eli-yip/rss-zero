package db

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type Post struct {
	ID       string    `gorm:"column:id;type:text;primaryKey"`
	PaperID  string    `gorm:"column:paper_id;type:text"`
	CreateAt time.Time `gorm:"column:create_at;type:timestamptz"`
	Title    string    `gorm:"column:title;type:text"`
	Text     string    `gorm:"column:text;type:text"`
	Raw      []byte    `gorm:"column:raw;type:bytea"`
}

func (p *Post) TableName() string { return "xiaobot_post" }

type DBPost interface {
	SavePost(post *Post) (err error)
	GetLatestTime(paperID string) (t time.Time, err error)
	GetLatestNPost(paperID string, n int) ([]Post, error)
}

func (d *DBService) SavePost(post *Post) (err error) { return d.Save(post).Error }

func (d *DBService) GetLatestTime(paperID string) (t time.Time, err error) {
	var post Post
	if err = d.Where("paper_id = ?", paperID).Order("create_at desc").First(&post).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return post.CreateAt, err
}

func (d *DBService) GetLatestNPost(paperID string, n int) ([]Post, error) {
	posts := make([]Post, 0, n)
	err := d.Where("paper_id = ?", paperID).Order("create_at desc").Limit(n).Find(&posts).Error
	return posts, err
}
