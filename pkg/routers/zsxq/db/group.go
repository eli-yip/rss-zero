package db

import (
	"time"

	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db/models"
)

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
	var groups []models.Group
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
	var group models.Group
	if err := s.db.Where("id = ?", gid).First(&group).Error; err != nil {
		return "", err
	}
	return group.Name, nil
}

func (s *ZsxqDBService) UpdateCrawlTime(gid int, t time.Time) error {
	var group models.Group
	if err := s.db.Where("id = ?", gid).First(&group).Error; err != nil {
		return err
	}
	group.UpdateAt = t
	return s.db.Save(&group).Error
}

func (s *ZsxqDBService) GetCrawlStatus(gid int) (finished bool, err error) {
	var group models.Group
	if err := s.db.Where("id = ?", gid).First(&group).Error; err != nil {
		return false, err
	}
	return group.Finished, nil
}

func (s *ZsxqDBService) SaveCrawlStatus(gid int, finished bool) error {
	var group models.Group
	if err := s.db.Where("id = ?", gid).First(&group).Error; err != nil {
		return err
	}
	group.Finished = finished
	return s.db.Save(&group).Error
}
