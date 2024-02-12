package db

import (
	"gorm.io/gorm"
)

type DB interface {
	DBTopic
	DBArticle
	DBObject
	DBAuthor
	DBGroup
}

type ZsxqDBService struct{ db *gorm.DB }

func NewZsxqDBService(db *gorm.DB) DB { return &ZsxqDBService{db: db} }
