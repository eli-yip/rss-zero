package db

type Article struct {
	ID    string `gorm:"column:id;type:text;primary_key"`
	URL   string `gorm:"column:url;type:text"`
	Title string `gorm:"column:title;type:text"`
	Text  string `gorm:"column:text;type:text"`
	Raw   []byte `gorm:"column:raw;type:bytea"`
}

func (a *Article) TableName() string { return "zsxq_article" }

type DBArticle interface {
	// Save article to zsxq_article table
	SaveArticle(article *Article) error
	// Get article
	GetArticle(articleID string) (*Article, error)
	// Get article text by id from zsxq_article table
	GetArticleText(articleID string) (text string, err error)
}

func (s *ZsxqDBService) SaveArticle(article *Article) error { return s.db.Save(article).Error }

func (s *ZsxqDBService) GetArticle(articleID string) (*Article, error) {
	var article Article
	if err := s.db.Where("id = ?", articleID).First(&article).Error; err != nil {
		return nil, err
	}
	return &article, nil
}

func (s *ZsxqDBService) GetArticleText(articleID string) (string, error) {
	var article Article
	if err := s.db.Where("id = ?", articleID).First(&article).Error; err != nil {
		return "", err
	}
	return article.Text, nil
}
