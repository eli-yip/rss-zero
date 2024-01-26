package db

import "gorm.io/gorm"

type DB interface {
	DataBaseAnswer
	DataBaseQuestion
	DataBasePost
	DataBasePin
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
	// Get author name
	GetAuthorName(id string) (name string, err error)
	CheckAuthorExist(id string) (exist bool, err error)
}

type DataBaseObject interface {
	// Save object info to zhihu_object table
	SaveObjectInfo(o *Object) error
	// Get object info from zhihu_object table
	GetObjectInfo(oid int) (o *Object, err error)
}

type DataBasePost interface {
	SavePost(p *Post) error
}

type DataBasePin interface {
	SavePin(p *Pin) error
}

type DBService struct{ *gorm.DB }

func NewDBService(db *gorm.DB) *DBService {
	return &DBService{db}
}

func (d *DBService) SavePost(p *Post) error {
	return d.Save(p).Error
}

func (d *DBService) SaveQuestion(q *Question) error {
	return d.Save(q).Error
}

func (d *DBService) SaveAuthor(a *Author) error {
	return d.Save(a).Error
}

func (d *DBService) GetAuthorName(id string) (name string, err error) {
	a := &Author{}
	err = d.Where("id = ?", id).First(a).Error
	return a.Name, err
}

func (d *DBService) CheckAuthorExist(id string) (exist bool, err error) {
	a := &Author{}
	err = d.Where("id = ?", id).First(a).Error
	return a.ID != "", err
}

func (d *DBService) SaveObjectInfo(o *Object) error {
	return d.Save(o).Error
}

func (d *DBService) GetObjectInfo(id int) (o *Object, err error) {
	o = &Object{}
	err = d.Where("id = ?", id).First(o).Error
	return
}

func (d *DBService) SavePin(p *Pin) error {
	return d.Save(p).Error
}
