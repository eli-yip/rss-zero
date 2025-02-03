package macked

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type DB interface {
	SaveTime(t time.Time) (err error)
	GetLatestTime() (t time.Time, err error)
}

type TimeInfo struct {
	ID         string    `gorm:"primaryKey"`
	LatestTime time.Time `gorm:"latest_time"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime"`
	DeleteAt   gorm.DeletedAt
}

func (*TimeInfo) TableName() string { return "macked_timeinfo" }

type DBService struct{ *gorm.DB }

func NewDBService(db *gorm.DB) *DBService { return &DBService{db} }

func (d *DBService) SaveTime(t time.Time) (err error) {
	err = d.Save(&TimeInfo{ID: "master", LatestTime: t}).Error
	return err
}

func (d *DBService) GetLatestTime() (t time.Time, err error) {
	var ti TimeInfo
	err = d.Find(&ti).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return time.Time{}, nil
	}
	return ti.LatestTime, err
}
