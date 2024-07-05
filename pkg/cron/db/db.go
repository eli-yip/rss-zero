package db

import "gorm.io/gorm"

type DBService struct{ *gorm.DB }

type DB interface {
	CronTaskIface
	CronJobIface
}

func NewDBService(db *gorm.DB) DB { return &DBService{db} }
