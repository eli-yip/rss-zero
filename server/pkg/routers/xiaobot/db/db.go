// package db contains the interface and implementation of xiaobot database operations
package db

import "gorm.io/gorm"

type DB interface {
	DBPaper
	DBPost
	DBCreator
}

type DBService struct{ *gorm.DB }

func NewDBService(db *gorm.DB) DB { return &DBService{db} }
