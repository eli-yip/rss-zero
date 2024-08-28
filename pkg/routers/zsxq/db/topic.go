package db

import (
	"time"

	"gorm.io/gorm"
)

type Topic struct {
	ID        int       `gorm:"column:id;primary_key"`
	Time      time.Time `gorm:"column:time"`
	GroupID   int       `gorm:"column:group_id"`
	Type      string    `gorm:"column:type;type:text"`
	Digested  bool      `gorm:"column:digested;type:bool"`
	AuthorID  int       `gorm:"column:author_id"`
	ShareLink string    `gorm:"column:share_link;type:text"`
	Title     *string   `gorm:"column:title;type:text"`
	Text      string    `gorm:"column:text;type:text"`
	Raw       []byte    `gorm:"column:raw;type:bytea"`
}

func (t *Topic) TableName() string { return "zsxq_topic" }

type DBTopic interface {
	// Save topic to zsxq_topic table
	SaveTopic(t *Topic) error
	// Get latest topic time from zsxq_topic table
	GetLatestTopicTime(gid int) (t time.Time, err error)
	// Get earliest topic time from zsxq_topic table
	GetEarliestTopicTime(gid int) (t time.Time, err error)
	// Get latest n topics from zsxq_topic table
	GetLatestNTopics(gid int, n int) (ts []Topic, err error)
	// Get All ids from zsxq_topic table
	GetAllTopicIDs(gid int) (ids []int, err error)
	// Fetch n topics before time from zsxq_topic table
	FetchNTopicsBefore(gid int, n int, t time.Time) (ts []Topic, err error)
	// Fetch n topics with options from zsxq_topic table
	FetchNTopics(n int, opt Options) (ts []Topic, err error)
	// Get topic by id from zsxq_topic table
	GetTopicByID(id int) (t Topic, err error)
	// Get topic id by share link from zsxq_topic table
	GetTopicIDByShareLink(shareLink string) (id int, err error)
}

func (s *ZsxqDBService) SaveTopic(t *Topic) error {
	return s.db.Save(t).Error
}

func (s *ZsxqDBService) GetLatestTopicTime(gid int) (time.Time, error) {
	var topic Topic
	if err := s.db.Where("group_id = ?", gid).Order("time desc").First(&topic).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return topic.Time, nil
}

func (s *ZsxqDBService) GetTopicByID(id int) (t Topic, err error) {
	err = s.db.Where("id = ?", id).First(&t).Error
	return t, err
}

func (s *ZsxqDBService) GetEarliestTopicTime(gid int) (time.Time, error) {
	var topic Topic
	if err := s.db.Where("group_id = ?", gid).Order("time asc").First(&topic).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}
	return topic.Time, nil
}

func (s *ZsxqDBService) GetLatestNTopics(gid, n int) (ts []Topic, err error) {
	err = s.db.Where("group_id = ?", gid).Order("time desc").Limit(n).Find(&ts).Error
	return ts, err
}

func (s *ZsxqDBService) FetchNTopicsBefore(gid, n int, t time.Time) (ts []Topic, err error) {
	err = s.db.Where("group_id = ? and time < ?", gid, t).Order("time desc").Limit(n).Find(&ts).Error
	return ts, err
}

func (s *ZsxqDBService) GetAllTopicIDs(gid int) (ids []int, err error) {
	var topics []Topic
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

func (s *ZsxqDBService) FetchNTopics(n int, opt Options) (ts []Topic, err error) {
	ts = make([]Topic, 0, n)

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

func (s *ZsxqDBService) GetTopicIDByShareLink(shareLink string) (int, error) {
	var topic Topic
	if err := s.db.Where("share_link = ?", shareLink).First(&topic).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, nil
		}
		return 0, err
	}
	return topic.ID, nil
}
