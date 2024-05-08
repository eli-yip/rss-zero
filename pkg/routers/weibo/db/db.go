package db

import "gorm.io/gorm"

type DB interface {
	DBObject
}

type DBService struct{ *gorm.DB }

func NewDBService(db *gorm.DB) DB { return &DBService{DB: db} }
