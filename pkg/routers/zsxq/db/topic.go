package db

import (
	"time"

	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db/models"
	"gorm.io/gorm"
)

func (s *ZsxqDBService) SaveTopic(t *models.Topic) error {
	return s.db.Save(t).Error
}

func (s *ZsxqDBService) GetLatestTopicTime(gid int) (time.Time, error) {
	var topic models.Topic
	if err := s.db.Where("group_id = ?", gid).Order("time desc").First(&topic).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return topic.Time, nil
}

func (s *ZsxqDBService) GetEarliestTopicTime(gid int) (time.Time, error) {
	var topic models.Topic
	if err := s.db.Where("group_id = ?", gid).Order("time asc").First(&topic).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return topic.Time, nil
}

func (s *ZsxqDBService) GetLatestNTopics(gid, n int) (ts []models.Topic, err error) {
	err = s.db.Where("group_id = ?", gid).Order("time desc").Limit(n).Find(&ts).Error
	return ts, err
}

func (s *ZsxqDBService) FetchNTopicsBeforeTime(gid, n int, t time.Time) (ts []models.Topic, err error) {
	err = s.db.Where("group_id = ? and time < ?", gid, t).Order("time desc").Limit(n).Find(&ts).Error
	return ts, err
}

func (s *ZsxqDBService) GetAllTopicIDs(gid int) (ids []int, err error) {
	var topics []models.Topic
	if err := s.db.Where("group_id = ?", gid).Order("time desc").Find(&topics).Error; err != nil {
		return nil, err
	}
	for _, topic := range topics {
		ids = append(ids, topic.ID)
	}
	return ids, nil
}

type Options struct {
	GroupID   int
	Type      *string
	Aid       *int
	Digested  *bool
	StartTime time.Time
	EndTime   time.Time
}

func (s *ZsxqDBService) FetchNTopics(n int, opt Options) (ts []models.Topic, err error) {
	ts = make([]models.Topic, 0, n)

	query := s.db.Limit(n).Where("group_id = ?", opt.GroupID)

	if opt.Type != nil {
		query = query.Where("type = ?", *opt.Type)
	}

	if opt.Aid != nil {
		query = query.Where("author_id = ?", *opt.Aid)
	}

	if opt.Digested != nil {
		query = query.Where("digested = ?", *opt.Digested)
	}

	if !opt.StartTime.IsZero() {
		query = query.Where("time >= ?", opt.StartTime)
	}

	if !opt.EndTime.IsZero() {
		query = query.Where("time <= ?", opt.EndTime)
	}

	if err := query.Order("time asc").Find(&ts).Error; err != nil {
		return nil, err
	}

	return ts, nil
}
