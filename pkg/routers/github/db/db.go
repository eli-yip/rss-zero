package db

import "gorm.io/gorm"

type DBService struct{ *gorm.DB }

func NewDBService(db *gorm.DB) DB { return &DBService{db} }

type DB interface {
	DBRelease
	DBSub
}
