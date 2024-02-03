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
}

type DBAuthor interface {
	// Save author info to zhihu_author table
	SaveAuthor(a *Author) error
	// Get author name
	GetAuthorName(id string) (name string, err error)
}

type DBObject interface {
	// Save object info to zhihu_object table
	SaveObjectInfo(o *Object) error
	// Get object info from zhihu_object table
	GetObjectInfo(oid int) (o *Object, err error)
}

type DBService struct{ *gorm.DB }

func NewDBService(db *gorm.DB) *DBService {
	return &DBService{db}
}

func (d *DBService) SaveAuthor(a *Author) error {
	return d.Save(a).Error
}

func (d *DBService) GetAuthorName(id string) (name string, err error) {
	a := &Author{}
	err = d.Where("id = ?", id).First(a).Error
	return a.Name, err
}

func (d *DBService) SaveObjectInfo(o *Object) error {
	return d.Save(o).Error
}

func (d *DBService) GetObjectInfo(id int) (o *Object, err error) {
	o = &Object{}
	err = d.Where("id = ?", id).First(o).Error
	return
}

type FetchOptionBase struct {
	UserID    *string
	StartTime time.Time
	EndTime   time.Time
}
