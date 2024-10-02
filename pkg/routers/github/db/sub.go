package db

import "gorm.io/gorm"

type Sub struct {
	ID         string `gorm:"column:id"`
	RepoID     string `gorm:"column:repo_id;primaryKey"`
	PreRelease bool   `gorm:"primaryKey;column:pre_release"`
	DeletedAt  gorm.DeletedAt
}

func (*Sub) TableName() string { return "github_subs" }

type DBSub interface {
	SaveSub(sub *Sub) error
	GetSub(repoID string, preRelease bool) (*Sub, error)
	GetSubIncludeDeleted(repoID string, preRelease bool) (*Sub, error)
	GetSubByID(id string) (*Sub, error)
	GetSubByIDIncludeDeleted(id string) (*Sub, error)
	GetSubs() ([]Sub, error)
	GetSubsIncludeDeleted() ([]Sub, error)
	DeleteSub(id string) error
	ActivateSub(id string) error
}

func (s *DBService) SaveSub(sub *Sub) error { return s.Save(sub).Error }

func (s *DBService) GetSub(repoID string, preRelease bool) (*Sub, error) {
	var sub Sub
	if err := s.Where("repo_id = ? AND pre_release = ?", repoID, preRelease).First(&sub).Error; err != nil {
		return nil, err
	}
	return &sub, nil
}

func (s *DBService) GetSubIncludeDeleted(repoID string, preRelease bool) (*Sub, error) {
	var sub Sub
	if err := s.Unscoped().Where("repo_id = ? AND pre_release = ?", repoID, preRelease).First(&sub).Error; err != nil {
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

func (s *DBService) GetSubByIDIncludeDeleted(id string) (*Sub, error) {
	var sub Sub
	if err := s.Unscoped().Where("id = ?", id).First(&sub).Error; err != nil {
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

func (s *DBService) GetSubsIncludeDeleted() (subs []Sub, err error) {
	subs = make([]Sub, 0)
	if err = s.Unscoped().Find(&subs).Error; err != nil {
		return nil, err
	}
	return subs, nil
}

func (s *DBService) DeleteSub(id string) error {
	return s.Where("id = ?", id).Delete(&Sub{}).Error
}

func (s *DBService) ActivateSub(id string) error {
	return s.Unscoped().Model(&Sub{}).Where("id = ?", id).Update("deleted_at", nil).Error
}
