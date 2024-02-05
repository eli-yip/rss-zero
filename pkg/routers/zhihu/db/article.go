package db

import (
	"time"

	"gorm.io/gorm"
)

type Article struct {
	ID       int       `gorm:"column:id;type:int;primary_key"`
	AuthorID string    `gorm:"column:author_id;type:text"`
	CreateAt time.Time `gorm:"column:create_at;type:timestamptz"`
	Title    string    `gorm:"column:title;type:text"`
	Text     string    `gorm:"column:text;type:text"`
	Raw      []byte    `gorm:"column:raw;type:bytea"`
}

func (p *Article) TableName() string { return "zhihu_article" }

type DBArticle interface {
	SaveArticle(p *Article) error
	GetLatestNArticle(n int, authorID string) ([]Article, error)
	GetLatestArticleTime(authorID string) (time.Time, error)
	FetchNArticle(n int, opt FetchArticleOption) (as []Article, err error)
	CountArticle(authorID string) (int, error)
	FetchNArticlesBeforeTime(n int, t time.Time, authorID string) (as []Article, err error)
}

func (d *DBService) SaveArticle(p *Article) error { return d.Save(p).Error }

func (d *DBService) GetLatestNArticle(n int, authorID string) ([]Article, error) {
	as := make([]Article, 0, n)
	if err := d.Where("author_id = ?", authorID).Order("create_at desc").Limit(n).Find(&as).Error; err != nil {
		return nil, err
	}
	return as, nil
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

func (d *DBService) FetchNArticlesBeforeTime(n int, t time.Time, authorID string) (as []Article, err error) {
	err = d.Where("author_id = ? and create_at < ?", authorID, t).Order("create_at desc").Limit(n).Find(&as).Error
	return as, err
}

func (d *DBService) CountArticle(authorID string) (int, error) {
	var count int64
	if err := d.Model(&Article{}).Where("author_id = ?", authorID).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

type FetchArticleOption struct{ FetchOptionBase }

func (d *DBService) FetchNArticle(n int, opt FetchArticleOption) (as []Article, err error) {
	as = make([]Article, 0, n)

	query := d.Limit(n)

	if opt.UserID != nil {
		query = query.Where("author_id = ?", *opt.UserID)
	}

	if !opt.StartTime.IsZero() {
		query = query.Where("create_at >= ?", opt.StartTime)
	}

	if !opt.EndTime.IsZero() {
		query = query.Where("create_at <= ?", opt.EndTime)
	}

	if err := query.Order("create_at asc").Find(&as).Error; err != nil {
		return nil, err
	}

	return as, nil
}
