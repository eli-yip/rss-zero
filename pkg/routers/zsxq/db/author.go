package db

import "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db/models"

func (s *ZsxqDBService) SaveAuthorInfo(a *models.Author) error {
	return s.db.Save(a).Error
}

func (s *ZsxqDBService) GetAuthorID(name string) (int, error) {
	var author models.Author
	if err := s.db.Where("alias = ? or name = ?", name, name).First(&author).Error; err != nil {
		return 0, err
	}
	return author.ID, nil
}

func (s *ZsxqDBService) GetAuthorName(aid int) (string, error) {
	var author models.Author
	if err := s.db.Where("id = ?", aid).First(&author).Error; err != nil {
		return "", err
	}
	return author.Name, nil
}
