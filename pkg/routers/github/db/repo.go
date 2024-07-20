package db

type Repo struct {
	ID         string `gorm:"column:id"`
	GithubUser string `gorm:"primaryKey;column:gh_user"`
	Name       string `gorm:"primaryKey"`
}

func (*Repo) TableName() string { return "github_repos" }

type DBRepo interface {
	SaveRepo(repo *Repo) error
	GetRepo(user, repoName string) (*Repo, error)
	GetRepoByID(id string) (*Repo, error)
	GetRepos() ([]Repo, error)
}

func (s *DBService) SaveRepo(repo *Repo) error { return s.Save(repo).Error }

func (s *DBService) GetRepo(user, repoName string) (*Repo, error) {
	var r Repo
	if err := s.Where("gh_user = ? AND repo_name = ?", user, repoName).First(&r).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *DBService) GetRepoByID(id string) (*Repo, error) {
	var r Repo
	if err := s.Where("id = ?", id).First(&r).Error; err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *DBService) GetRepos() (repos []Repo, err error) {
	repos = make([]Repo, 0)
	if err = s.Find(&repos).Error; err != nil {
		return nil, err
	}
	return repos, nil
}
