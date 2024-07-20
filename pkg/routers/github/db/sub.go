package db

type Sub struct {
	ID         string `gorm:"column:id"`
	GithubUser string `gorm:"primaryKey;column:gh_user"`
	Repo       string `gorm:"primaryKey"`
	PreRelease bool   `gorm:"primaryKey;column:pre_release"`
}

func (*Sub) TableName() string { return "github_subs" }

type DBSub interface {
	SaveSub(sub *Sub) error
	GetSub(user, repo string, preRelease bool) (*Sub, error)
	GetSubByID(id string) (*Sub, error)
	GetSubs() ([]Sub, error)
}

func (s *DBService) SaveSub(sub *Sub) error { return s.Save(sub).Error }

func (s *DBService) GetSub(user, repo string, preRelease bool) (*Sub, error) {
	var sub Sub
	if err := s.Where("gh_user = ? AND repo = ? AND pre_release = ?", user, repo, preRelease).First(&sub).Error; err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *DBService) GetSubByID(id string) (*Sub, error) {
	var sub Sub
	if err := s.Where("id = ?", id).First(&sub).Error; err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *DBService) GetSubs() (subs []Sub, err error) {
	subs = make([]Sub, 0)
	if err = s.Find(&subs).Error; err != nil {
		return nil, err
	}
	return subs, nil
}
