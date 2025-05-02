package db

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/xid"
	"gorm.io/gorm"
)

type Sub struct {
	ID        string `gorm:"column:id;type:string"`
	AuthorID  string `gorm:"column:author_id;type:text;primary_key"`
	Type      int    `gorm:"column:type;type:int;primary_key"`
	Finished  bool   `gorm:"column:finished;type:boolean"`
	DeletedAt gorm.DeletedAt
}

func (s *Sub) TableName() string { return "zhihu_sub" }

type DBSub interface {
	AddSub(authorID string, subType int) error
	CheckSub(authorID string, subType int) (bool, error)
	CheckSubIncludeDeleted(authorID string, subType int) (bool, error)
	CheckSubByID(id string) (bool, error)
	GetSubs() ([]Sub, error)
	GetSubsIncludeDeleted() ([]Sub, error)
	GetSubsWithNoID() ([]Sub, error)
	SetSubID(authorID string, subType int, id string) error
	SetStatus(authorID string, subType int, finished bool) error
	DeleteSub(id string) error
	ActivateSub(id string) error
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

func (d *DBService) CheckSubIncludeDeleted(authorID string, subType int) (bool, error) {
	var sub Sub
	if err := d.Unscoped().Where("author_id = ? and type = ?", authorID, subType).First(&sub).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (d *DBService) CheckSubByID(id string) (bool, error) {
	var sub Sub
	if err := d.Where("id = ?", id).First(&sub).Error; err != nil {
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

func (d *DBService) GetSubsIncludeDeleted() ([]Sub, error) {
	var subs []Sub
	// order by id
	if err := d.Unscoped().Order("id ASC").Find(&subs).Error; err != nil {
		return nil, err
	}
	return subs, nil
}

func (d *DBService) GetSubsWithNoID() ([]Sub, error) {
	var subs []Sub
	if err := d.Where("id = ? or id IS NULL", "").Find(&subs).Error; err != nil {
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

func SetEmptySubID(db *gorm.DB) (n int, err error) {
	zhihuDBService := NewDBService(db)
	subs, err := zhihuDBService.GetSubsWithNoID()
	if err != nil {
		return 0, fmt.Errorf("failed to get subs with no id: %w", err)
	}

	for _, sub := range subs {
		if err := zhihuDBService.SetSubID(sub.AuthorID, sub.Type, xid.New().String()); err != nil {
			return 0, fmt.Errorf("failed to set sub id: %w", err)
		}
		n++
		time.Sleep(100 * time.Millisecond)
	}

	return n, nil
}

func (d *DBService) DeleteSub(id string) (err error) {
	return d.Where("id = ?", id).Delete(&Sub{}).Error
}

func (d *DBService) ActivateSub(id string) (err error) {
	return d.Unscoped().Model(&Sub{}).Where("id = ?", id).Update("deleted_at", nil).Error
}
