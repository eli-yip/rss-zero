package db

import (
	"time"

	"gorm.io/gorm"
)

type Pin struct {
	ID       int       `gorm:"column:id;type:int;primary_key"`
	AuthorID string    `gorm:"column:author_id;type:string"`
	CreateAt time.Time `gorm:"column:create_at;type:timestamp with time zone"`
	Text     string    `gorm:"column:text;type:text"`
	Raw      []byte    `gorm:"column:raw;type:bytea"`
}

func (p *Pin) TableName() string { return "zhihu_pin" }

type DBPin interface {
	SavePin(p *Pin) error
	GetLatestPinTime(userID string) (time.Time, error)
	FetchNPin(n int, opt FetchPinOption) (ps []Pin, err error)
}

func (d *DBService) SavePin(p *Pin) error { return d.Save(p).Error }

func (d *DBService) GetLatestPinTime(userID string) (time.Time, error) {
	var t time.Time
	if err := d.Model(&Pin{}).Where("author_id = ?", userID).Order("create_at desc").Limit(1).Pluck("create_at", &t).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return t, nil
}

type FetchPinOption struct{ FetchOptionBase }

func (d *DBService) FetchNPin(n int, opt FetchPinOption) (ps []Pin, err error) {
	ps = make([]Pin, 0, n)

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

	if err := query.Order("create_at asc").Find(&ps).Error; err != nil {
		return nil, err
	}

	return ps, nil
}
