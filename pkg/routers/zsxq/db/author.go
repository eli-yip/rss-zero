package db

type Author struct {
	ID    int     `gorm:"column:id;primary_key"`
	Name  string  `gorm:"column:name;type:text"`
	Alias *string `gorm:"column:alias;type:text"`
}

func (a *Author) TableName() string { return "zsxq_author" }

type DBAuthor interface {
	// Save author info to zsxq_author table
	SaveAuthorInfo(a *Author) error
	// Get author name by id from zsxq_author table
	GetAuthorName(aid int) (name string, err error)
	// Get author id by name or alias from zsxq_author table
	GetAuthorID(name string) (id int, err error)
}

func (s *ZsxqDBService) SaveAuthorInfo(a *Author) error {
	return s.db.Save(a).Error
}

func (s *ZsxqDBService) GetAuthorID(name string) (int, error) {
	var author Author
	if err := s.db.Where("alias = ? or name = ?", name, name).First(&author).Error; err != nil {
		return 0, err
	}
	return author.ID, nil
}

func (s *ZsxqDBService) GetAuthorName(aid int) (string, error) {
	var author Author
	if err := s.db.Where("id = ?", aid).First(&author).Error; err != nil {
		return "", err
	}
	return author.Name, nil
}
