package db

import (
	"fmt"
	"math/rand"
	"time"

	"gorm.io/gorm"
)

type Topic struct {
	ID       int       `gorm:"column:id;primary_key"`
	Time     time.Time `gorm:"column:time"`
	GroupID  int       `gorm:"column:group_id"`
	Type     string    `gorm:"column:type;type:text"`
	Digested bool      `gorm:"column:digested;type:bool"`
	AuthorID int       `gorm:"column:author_id"`
	Title    *string   `gorm:"column:title;type:text"` // Although title is not null in q&a and talk, it is null in some topics
	Raw      []byte    `gorm:"column:raw;type:bytea"`
}

func (t *Topic) TableName() string { return "zsxq_topic" }

type DBTopic interface {
	// SaveTopicTx 在单事务内提交一条 topic 产生的全部行；原子性来自事务本身（一起提交或
	// 一起回滚），根行最后写只是可读性约定，无 FK 强制、不改变回滚语义。
	SaveTopicTx(root *Topic, author *Author, article *Article, objects []Object) error
	// Get latest topic time from zsxq_topic table
	GetLatestTopicTime(gid int) (t time.Time, err error)
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
	// Random select n topics from zsxq_topic table
	RandomSelect(userID, n int, digest bool) (topics []Topic, err error)
}

// SaveTopicTx 把一条 topic 解析出的全部事实行放进同一个事务提交：作者、外部文章、各对象、
// topic 根行。原子性来自事务本身（任一步失败整体回滚），消除「资源先写、根行后写、中途失败
// 留半态」的旧 bug；根行最后写只是可读性约定，无 FK 强制、不改变回滚语义。
//
// 对象二进制已在事务外上传 OSS（见 parse.collect*），只有上传成功的对象元数据才会进入
// objects，故这里只做纯 DB 写。author/article 为 nil（如未知类型）时相应步骤跳过。
func (s *ZsxqDBService) SaveTopicTx(root *Topic, author *Author, article *Article, objects []Object) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if author != nil {
			if err := tx.Save(author).Error; err != nil {
				return fmt.Errorf("failed to save author %d: %w", author.ID, err)
			}
		}
		if article != nil {
			if err := tx.Save(article).Error; err != nil {
				return fmt.Errorf("failed to save article %s: %w", article.ID, err)
			}
		}
		for i := range objects {
			if err := tx.Save(&objects[i]).Error; err != nil {
				return fmt.Errorf("failed to save object %d: %w", objects[i].ID, err)
			}
		}
		if err := tx.Save(root).Error; err != nil {
			return fmt.Errorf("failed to save topic root %d: %w", root.ID, err)
		}
		return nil
	})
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

func (s *ZsxqDBService) RandomSelect(userID, n int, digest bool) (topics []Topic, err error) {
	topics = make([]Topic, 0, n)

	topicIDs := make([]int, 0, n)
	s.db.Model(&Topic{}).Where("author_id = ? AND digested = ?", userID, digest).Pluck("id", &topicIDs)

	if len(topicIDs) <= n {
		if err = s.db.Where("id in ?", topicIDs).Find(&topics).Error; err != nil {
			return nil, fmt.Errorf("failed to get topics: %w", err)
		}
		return topics, nil
	}

	rand.Shuffle(len(topicIDs), func(i, j int) {
		topicIDs[i], topicIDs[j] = topicIDs[j], topicIDs[i]
	})

	topicIDs = topicIDs[:n]

	if err := s.db.Where("id in ?", topicIDs).Find(&topics).Error; err != nil {
		return nil, fmt.Errorf("failed to get topics: %w", err)
	}

	return topics, nil
}
