package db

import (
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db/models"
	"gorm.io/gorm"
)

type DataBaseIface interface {
	SaveTopic(*models.Topic) error
	SaveObject(*models.Object) error
	GetObjectInfo(id int) (*models.Object, error)
}

type ZsxqDBService struct{ db *gorm.DB }

func NewZsxqDBService(db *gorm.DB) *ZsxqDBService { return &ZsxqDBService{db: db} }

func (s *ZsxqDBService) SaveTopic(topic *models.Topic) error {
	return s.db.Save(topic).Error
}

func (s *ZsxqDBService) SaveObject(object *models.Object) error {
	return s.db.Save(object).Error
}

func (s *ZsxqDBService) GetObjectInfo(id int) (*models.Object, error) {
	var object models.Object
	if err := s.db.First(&object, id).Error; err != nil {
		return nil, err
	}
	return &object, nil
}
