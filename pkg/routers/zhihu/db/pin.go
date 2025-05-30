package db

import (
	"slices"
	"time"

	"gorm.io/gorm"
)

type Pin struct {
	ID       int       `gorm:"column:id;type:int;primary_key"`
	AuthorID string    `gorm:"column:author_id;type:string"`
	CreateAt time.Time `gorm:"column:create_at;type:timestamptz"`
	UpdateAt time.Time `gorm:"column:update_at;type:timestamptz"`
	Title    string    `gorm:"column:title;type:text"`
	Text     string    `gorm:"column:text;type:text"`
	Raw      []byte    `gorm:"column:raw;type:bytea"`
}

func (p *Pin) TableName() string { return "zhihu_pin" }

type DBPin interface {
	GetPin(id int) (*Pin, error)
	SavePin(p *Pin) error
	GetLatestNPin(n int, authorID string) ([]Pin, error)
	GetLatestPinTime(userID string) (time.Time, error)
	FetchNPin(n int, opt FetchPinOption) (ps []Pin, err error)
	FetchPinByIDs(ids []int) (map[int]Pin, error)
	FetchPinWithDateRange(authorID string, limit, offset, order int, startTime, endTime time.Time) (ps []Pin, err error)
	GetPinAfter(authorID string, t time.Time) ([]Pin, error)
	CountPin(authorID string) (int, error)
	CountPinWithDateRange(authorID string, startTime, endTime time.Time) (int, error)
	FetchNPinsBeforeTime(n int, t time.Time, authorID string) (ps []Pin, err error)
}

func (d *DBService) GetPin(id int) (*Pin, error) {
	var p Pin
	if err := d.First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (d *DBService) SavePin(p *Pin) error { return d.Save(p).Error }

func (d *DBService) GetLatestNPin(n int, authorID string) ([]Pin, error) {
	ps := make([]Pin, 0, n)
	if err := d.Where("author_id = ?", authorID).Order("create_at desc").Limit(n).Find(&ps).Error; err != nil {
		return nil, err
	}
	return ps, nil
}

func (d *DBService) FetchNPinsBeforeTime(n int, t time.Time, authorID string) (ps []Pin, err error) {
	err = d.Where("author_id = ? and create_at < ?", authorID, t).Order("create_at desc").Limit(n).Find(&ps).Error
	return ps, err
}

func (d *DBService) FetchPinByIDs(ids []int) (map[int]Pin, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	ps := make([]Pin, 0, len(ids))
	if err := d.Where("id IN ?", ids).Find(&ps).Error; err != nil {
		return nil, err
	}

	result := make(map[int]Pin, len(ps))
	for pin := range slices.Values(ps) {
		result[pin.ID] = pin
	}

	return result, nil
}

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

func (d *DBService) CountPin(authorID string) (int, error) {
	var count int64
	if err := d.Model(&Pin{}).Where("author_id = ?", authorID).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
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

func (d *DBService) FetchPinWithDateRange(authorID string, limit, offset, order int, startTime, endTime time.Time) (ps []Pin, err error) {
	ps = make([]Pin, 0, limit)
	stmt := d.Where("author_id = ? and create_at >= ? and create_at < ?", authorID, startTime, endTime).Order("create_at desc").Limit(limit).Offset(offset)
	if order == 0 {
		stmt = stmt.Order("create_at desc")
	} else {
		stmt = stmt.Order("create_at asc")
	}
	if err := stmt.Find(&ps).Error; err != nil {
		return nil, err
	}
	return ps, nil
}

func (d *DBService) GetPinAfter(authorID string, t time.Time) ([]Pin, error) {
	var ps []Pin
	if err := d.Where("author_id = ? and create_at > ?", authorID, t).Order("create_at asc").Find(&ps).Error; err != nil {
		return nil, err
	}
	return ps, nil
}

func (d *DBService) CountPinWithDateRange(authorID string, startTime, endTime time.Time) (int, error) {
	var count int64
	if err := d.Model(&Pin{}).Where("author_id = ? and create_at >= ? and create_at < ?", authorID, startTime, endTime).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}
