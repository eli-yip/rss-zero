package db

import "time"

type Release struct {
	ID          int       `gorm:"primaryKey"`
	SubID       string    `gorm:"column:sub_id"`
	URL         string    `gorm:"column:url"`
	Tag         string    `gorm:"column:tag"`
	Title       string    `gorm:"column:title"`
	Body        string    `gorm:"column:body"`
	Excerpt     string    `gorm:"column:excerpt"`
	PreRelease  bool      `gorm:"column:pre_release"`
	PublishedAt time.Time `gorm:"column:published_at"`
}

func (*Release) TableName() string { return "github_releases" }

type DBRelease interface {
	SaveRelease(release *Release) error
	GetRelease(id int) (*Release, error)
	GetReleaseByTag(subID, tag string) (*Release, error)
	GetReleases(subID string, preRelease bool, page, pageSize int) ([]Release, error)
}

func (s *DBService) SaveRelease(release *Release) error { return s.Save(release).Error }

func (s *DBService) GetRelease(id int) (*Release, error) {
	var release Release
	if err := s.First(&release, id).Error; err != nil {
		return nil, err
	}
	return &release, nil
}

func (s *DBService) GetReleaseByTag(subID, tag string) (*Release, error) {
	var release Release
	if err := s.Where("sub_id = ? AND tag = ?", subID, tag).First(&release).Error; err != nil {
		return nil, err
	}
	return &release, nil
}

func (s *DBService) GetReleases(subID string, preRelease bool, page, pageSize int) (releases []Release, err error) {
	releases = make([]Release, 0)
	query := s.Where("sub_id = ?", subID).Order("published_at DESC").Offset((page - 1) * pageSize).Limit(pageSize)
	if !preRelease {
		query = query.Where("pre_release = ?", preRelease)
	}

	if err = query.Find(&releases).Error; err != nil {
		return nil, err
	}
	return releases, nil
}
