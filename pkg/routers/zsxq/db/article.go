package db

import "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db/models"

type DBArticle interface {
	// Save article to zsxq_article table
	SaveArticle(a *models.Article) error
	// Get article
	GetArticle(aid string) (a *models.Article, err error)
	// Get article text by id from zsxq_article table
	GetArticleText(aid string) (text string, err error)
}

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
