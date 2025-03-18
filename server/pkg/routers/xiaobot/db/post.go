package db

import (
	"errors"
	"fmt"
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
	// SavePost save a xiaobot post to db
	SavePost(post *Post) (err error)
	// GetLatestTime get the latest post time of a paper
	GetLatestTime(paperID string) (t time.Time, err error)
	// FetchNPost get n post of a paper, create time ascending
	FetchNPost(n int, opt Option) (ps []Post, err error)
	// FetchNPostBefore get n post of a paper before a time
	FetchNPostBefore(n int, paperID string, t time.Time) ([]Post, error)
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

// Option is the option for FetchNPost
type Option struct {
	PaperID   string    // paper id
	StartTime time.Time // start time, inclusive
	EndTime   time.Time // end time, inclusive
}

func (d *DBService) FetchNPost(n int, opt Option) (ps []Post, err error) {
	ps = make([]Post, 0, n)

	query := d.Limit(n).Where("paper_id = ?", opt.PaperID).Order("create_at asc")

	if !opt.StartTime.IsZero() {
		query = query.Where("create_at >= ?", opt.StartTime)
	}

	if !opt.EndTime.IsZero() {
		query = query.Where("create_at <= ?", opt.EndTime)
	}

	if err = query.Find(&ps).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch posts: %w", err)
	}

	return ps, nil
}

func (d *DBService) FetchNPostBefore(n int, paperID string, t time.Time) ([]Post, error) {
	posts := make([]Post, 0, n)
	err := d.Where("paper_id = ? AND create_at < ?", paperID, t).Order("create_at desc").Limit(n).Find(&posts).Error
	return posts, err
}
