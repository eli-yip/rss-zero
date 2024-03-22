package db

type Author struct {
	ID    int     `gorm:"column:id;primary_key"`
	Name  string  `gorm:"column:name;type:text"`
	Alias *string `gorm:"column:alias;type:text"`
}

func (a *Author) TableName() string { return "zsxq_author" }

type DBAuthor interface {
	// Save author info to zsxq_author table
	SaveAuthor(author *Author) error
	// Get author name by id from zsxq_author table
	GetAuthorName(authorID int) (authorName string, err error)
	// Get author id by name or alias from zsxq_author table
	GetAuthorID(authorName string) (authorID int, err error)
}

func (s *ZsxqDBService) SaveAuthor(author *Author) error { return s.db.Save(author).Error }

func (s *ZsxqDBService) GetAuthorName(authorID int) (authorName string, err error) {
	var author Author
	if err = s.db.Where("id = ?", authorID).First(&author).Error; err != nil {
		return "", err
	}
	return author.Name, nil
}

func (s *ZsxqDBService) GetAuthorID(authorName string) (authorID int, err error) {
	var author Author
	if err = s.db.Where("alias = ? or name = ?", authorName, authorName).First(&author).Error; err != nil {
		return 0, err
	}
	return author.ID, nil
}
