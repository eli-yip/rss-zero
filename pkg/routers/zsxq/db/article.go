package db

import (
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db/models"
)

func (s *ZsxqDBService) SaveArticle(a *models.Article) error {
	return s.db.Save(a).Error
}

func (s *ZsxqDBService) GetArticleText(aid string) (string, error) {
	var article models.Article
	if err := s.db.Where("id = ?", aid).First(&article).Error; err != nil {
		return "", err
	}
	return article.Text, nil
}

func (s *ZsxqDBService) GetArticle(aid string) (*models.Article, error) {
	var article models.Article
	if err := s.db.Where("id = ?", aid).First(&article).Error; err != nil {
		return nil, err
	}
	return &article, nil
}
