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
}

type DBService struct{ *gorm.DB }

func NewDBService(db *gorm.DB) *DBService { return &DBService{db} }

type FetchOptionBase struct {
	UserID    *string
	StartTime time.Time
	EndTime   time.Time
}

const (
	TypeAnswer = iota
	TypeArticle
	TypePin
)
