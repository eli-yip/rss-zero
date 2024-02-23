package db

import (
	"time"
)

type Group struct {
	ID         int       `gorm:"column:id;primary_key"`
	Name       string    `gorm:"column:name;type:text"`
	UpdateAt   time.Time `gorm:"column:update_at"`
	ErrorTimes int       `gorm:"column:error_times;type:int"`
	Finished   bool      `gorm:"column:finished;type:bool"`
}

func (g *Group) TableName() string { return "zsxq_group" }

type DBGroup interface {
	// Get all zsxq group ids from zsxq_group table
	GetZsxqGroupIDs() (ids []int, err error)
	// Get group name by group id from zsxq_group table
	GetGroupName(gid int) (name string, err error)
	// Save latest crawl time to zsxq_group table
	UpdateCrawlTime(gid int, t time.Time) (err error)
	// Get crawl status from zsxq_group table
	GetCrawlStatus(gid int) (finished bool, err error)
	// Save crawl status to zsxq_group table
	SaveCrawlStatus(gid int, finished bool) (err error)
}

func (s *ZsxqDBService) GetZsxqGroupIDs() ([]int, error) {
	var groups []Group
	if err := s.db.Find(&groups).Error; err != nil {
		return nil, err
	}

	var groupIDs []int
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
	}
	return groupIDs, nil
}

func (s *ZsxqDBService) GetGroupName(gid int) (name string, err error) {
	var group Group
	if err := s.db.Where("id = ?", gid).First(&group).Error; err != nil {
		return "", err
	}
	return group.Name, nil
}

func (s *ZsxqDBService) UpdateCrawlTime(gid int, t time.Time) error {
	var group Group
	if err := s.db.Where("id = ?", gid).First(&group).Error; err != nil {
		return err
	}
	group.UpdateAt = t
	return s.db.Save(&group).Error
}

func (s *ZsxqDBService) GetCrawlStatus(gid int) (finished bool, err error) {
	var group Group
	if err := s.db.Where("id = ?", gid).First(&group).Error; err != nil {
		return false, err
	}
	return group.Finished, nil
}

func (s *ZsxqDBService) SaveCrawlStatus(gid int, finished bool) error {
	var group Group
	if err := s.db.Where("id = ?", gid).First(&group).Error; err != nil {
		return err
	}
	group.Finished = finished
	return s.db.Save(&group).Error
}
