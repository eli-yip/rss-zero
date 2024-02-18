package db

type Article struct {
	ID    string `gorm:"column:id;primary_key"`
	URL   string `gorm:"column:url;type:text"`
	Title string `gorm:"column:title;type:text"`
	Text  string `gorm:"column:text;type:text"`
	Raw   []byte `gorm:"column:raw;type:bytea"`
}

func (a *Article) TableName() string { return "zsxq_article" }

type DBArticle interface {
	// Save article to zsxq_article table
	SaveArticle(a *Article) error
	// Get article
	GetArticle(aid string) (a *Article, err error)
	// Get article text by id from zsxq_article table
	GetArticleText(aid string) (text string, err error)
}

func (s *ZsxqDBService) SaveArticle(a *Article) error {
	return s.db.Save(a).Error
}

func (s *ZsxqDBService) GetArticleText(aid string) (string, error) {
	var article Article
	if err := s.db.Where("id = ?", aid).First(&article).Error; err != nil {
		return "", err
	}
	return article.Text, nil
}

func (s *ZsxqDBService) GetArticle(aid string) (*Article, error) {
	var article Article
	if err := s.db.Where("id = ?", aid).First(&article).Error; err != nil {
		return nil, err
	}
	return &article, nil
}
