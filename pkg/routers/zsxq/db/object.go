package db

import "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db/models"

type DBObject interface {
	// Save object info to zsxq_object table
	SaveObjectInfo(o *models.Object) error
	// Get object info from zsxq_object table
	GetObjectInfo(oid int) (o *models.Object, err error)
}

func (s *ZsxqDBService) SaveObjectInfo(o *models.Object) error {
	return s.db.Save(o).Error
}

func (s *ZsxqDBService) GetObjectInfo(oid int) (*models.Object, error) {
	var object models.Object
	if err := s.db.Where("id = ?", oid).First(&object).Error; err != nil {
		return nil, err
	}
	return &object, nil
}
