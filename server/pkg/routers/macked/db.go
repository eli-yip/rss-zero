package macked

import (
	"errors"
	"time"

	"github.com/rs/xid"
	"gorm.io/gorm"
)

type DB interface {
	SaveTime(t time.Time) (err error)
	GetLatestTime() (t time.Time, err error)

	CreateAppInfo(appName string) (appInfo *AppInfo, err error)
	IsAppInfoExists(appName string) (exists bool, err error)
	GetAppInfos() (infos []AppInfo, err error)
	DeleteAppInfo(id string) (err error)
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

func NewDBService(db *gorm.DB) DB { return &DBService{db} }

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

type AppInfo struct {
	ID string `gorm:"primaryKey"`

	AppName string `gorm:"column:app_name"`

	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
	DeleteAt  gorm.DeletedAt
}

func (*AppInfo) TableName() string { return "macked_appinfo" }

func (d *DBService) CreateAppInfo(appName string) (appInfo *AppInfo, err error) {
	appInfo = &AppInfo{
		ID:      xid.New().String(),
		AppName: appName,
	}
	err = d.Save(appInfo).Error
	return appInfo, err
}

func (d *DBService) GetAppInfos() (infos []AppInfo, err error) {
	err = d.Find(&infos).Error
	return
}

func (d *DBService) IsAppInfoExists(appName string) (exists bool, err error) {
	err = d.Where("app_name = ?", appName).First(&AppInfo{}).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return true, err
}

func (d *DBService) DeleteAppInfo(id string) (err error) {
	err = d.Delete(&AppInfo{ID: id}).Error
	return
}
