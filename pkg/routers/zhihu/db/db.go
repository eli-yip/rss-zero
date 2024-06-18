package db

import (
	"time"

	"gorm.io/gorm"
)

type DB interface {
	DBAnswer
	DBQuestion
	DBArticle
	DBPin
	DBAuthor
	DBObject
	DBSub
	EncryptionServiceIface
}

type DBService struct{ *gorm.DB }

func NewDBService(db *gorm.DB) DB { return &DBService{db} }

type FetchOptionBase struct {
	UserID    *string
	StartTime time.Time
	EndTime   time.Time
}
