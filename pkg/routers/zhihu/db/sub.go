package db

import (
	"errors"

	"github.com/rs/xid"
	"gorm.io/gorm"
)

type Sub struct {
	ID       string `gorm:"column:id;type:string"`
	AuthorID string `gorm:"column:author_id;type:text;primary_key"`
	Type     int    `gorm:"column:type;type:int;primary_key"`
	Finished bool   `gorm:"column:finished;type:boolean"`
}

func (s *Sub) TableName() string { return "zhihu_sub" }

type DBSub interface {
	AddSub(authorID string, subType int) error
	CheckSub(authorID string, subType int) (bool, error)
	GetSubs() ([]Sub, error)
	GetSubsWithNoID() ([]Sub, error)
	SetSubID(authorID string, subType int, id string) error
	SetStatus(authorID string, subType int, finished bool) error
}

func (d *DBService) AddSub(authorID string, subType int) error {
	return d.Save(&Sub{ID: xid.New().String(), AuthorID: authorID, Type: subType}).Error
}

func (d *DBService) CheckSub(authorID string, subType int) (bool, error) {
	var sub Sub
	if err := d.Where("author_id = ? and type = ?", authorID, subType).First(&sub).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (d *DBService) GetSubs() ([]Sub, error) {
	var subs []Sub
	// order by id
	if err := d.Order("id ASC").Find(&subs).Error; err != nil {
		return nil, err
	}
	return subs, nil
}

func (d *DBService) GetSubsWithNoID() ([]Sub, error) {
	var subs []Sub
	if err := d.Where("id = ?", "").Find(&subs).Error; err != nil {
		return nil, err
	}
	return subs, nil
}

func (d *DBService) SetSubID(authorID string, subType int, id string) error {
	return d.Model(&Sub{}).Where("author_id = ? and type = ?", authorID, subType).Update("id", id).Error
}

func (d *DBService) SetStatus(authorID string, subType int, finished bool) error {
	return d.Model(&Sub{}).Where("author_id = ? and type = ?", authorID, subType).Update("finished", finished).Error
}
