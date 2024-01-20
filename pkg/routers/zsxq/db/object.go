package db

import "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db/models"

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
