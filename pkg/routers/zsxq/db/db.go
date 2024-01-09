package db

import (
	"time"

	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db/models"
	"gorm.io/gorm"
)

type DataBaseIface interface {
	SaveTopic(*models.Topic) error
	SaveObject(*models.Object) error
	GetObjectInfo(id int) (*models.Object, error)
	GetZsxqGroupIDs() ([]int, error)
	GetLatestTopicTime(groupID int) (time.Time, error)
	SaveLatestTime(groupID int, latestTime time.Time) error
}

type ZsxqDBService struct{ db *gorm.DB }

func NewZsxqDBService(db *gorm.DB) *ZsxqDBService { return &ZsxqDBService{db: db} }

func (s *ZsxqDBService) SaveTopic(topic *models.Topic) error {
	return s.db.Save(topic).Error
}

func (s *ZsxqDBService) SaveObject(object *models.Object) error {
	return s.db.Save(object).Error
}

func (s *ZsxqDBService) GetObjectInfo(id int) (*models.Object, error) {
	var object models.Object
	if err := s.db.First(&object, id).Error; err != nil {
		return nil, err
	}
	return &object, nil
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

func (s *ZsxqDBService) GetLatestTopicTime(groupID int) (time.Time, error) {
	var topic models.Topic
	if err := s.db.Where("group_id = ?", groupID).Order("time desc").First(&topic).Error; err != nil {
		return time.Time{}, err
	}
	return topic.Time, nil
}

func (s *ZsxqDBService) SaveLatestTime(groupID int, latestTime time.Time) error {
	var group models.Group
	if err := s.db.First(&group, groupID).Error; err != nil {
		return err
	}
	group.UpdateAt = latestTime
	return s.db.Save(&group).Error
}
