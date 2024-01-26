package db

import "gorm.io/gorm"

type DB interface {
	DataBaseAnswer
	DataBaseQuestion
	DataBaseAuthor
	DataBaseObject
}

type DataBaseQuestion interface {
	// Save question info to zhihu_question table
	SaveQuestion(q *Question) error
}

type DataBaseAuthor interface {
	// Save author info to zhihu_author table
	SaveAuthor(a *Author) error
}

type DataBaseObject interface {
	// Save object info to zhihu_object table
	SaveObjectInfo(o *Object) error
	// Get object info from zhihu_object table
	GetObjectInfo(oid int) (o *Object, err error)
}

type DBService struct{ *gorm.DB }

func NewDBService(db *gorm.DB) *DBService {
	return &DBService{db}
}

func (d *DBService) SaveQuestion(q *Question) error {
	return d.Save(q).Error
}

func (d *DBService) SaveAuthor(a *Author) error {
	return d.Save(a).Error
}

func (d *DBService) SaveObjectInfo(o *Object) error {
	return d.Save(o).Error
}

func (d *DBService) GetObjectInfo(id int) (o *Object, err error) {
	o = &Object{}
	err = d.Where("id = ?", id).First(o).Error
	return
}
