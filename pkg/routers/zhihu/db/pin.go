package db

import "time"

type Pin struct {
	ID       int       `gorm:"column:id;type:int;primary_key"`
	AuthorID string    `gorm:"column:author_id;type:string"`
	CreateAt time.Time `gorm:"column:create_at;type:timestamp"`
	Text     string    `gorm:"column:text;type:text"`
	Raw      []byte    `gorm:"column:raw;type:bytea"`
}

func (p *Pin) TableName() string { return "zhihu_pin" }

type DataBasePin interface {
	SavePin(p *Pin) error
}

func (d *DBService) SavePin(p *Pin) error {
	return d.Save(p).Error
}
