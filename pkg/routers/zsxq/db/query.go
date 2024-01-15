package db

import (
	"time"

	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db/models"
	"gorm.io/gorm"
)

type Options struct {
	Type      *string
	Aid       *int
	Digested  *bool
	StartTime time.Time
	EndTime   time.Time
}

type ZsxqDBService struct{ db *gorm.DB }

func NewZsxqDBService(db *gorm.DB) *ZsxqDBService { return &ZsxqDBService{db: db} }

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

func (s *ZsxqDBService) FetchNTopics(n int, opt Options) (ts []models.Topic, err error) {
	ts = make([]models.Topic, 0, n)

	query := s.db.Limit(n)

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

func (s *ZsxqDBService) SaveArticle(a *models.Article) error {
	return s.db.Save(a).Error
}

func (s *ZsxqDBService) GetArticleText(aid string) (string, error) {
	var article models.Article
	if err := s.db.Where("id = ?", aid).First(&article).Error; err != nil {
		return "", err
	}
	return article.Text, nil
}

func (s *ZsxqDBService) SaveObjectInfo(o *models.Object) error {
	return s.db.Save(o).Error
}

func (s *ZsxqDBService) GetObjectInfo(oid int) (*models.Object, error) {
	var object models.Object
	if err := s.db.Where("id = ?", oid).First(&object).Error; err != nil {
		return nil, err
	}
	return &object, nil
}

func (s *ZsxqDBService) SaveAuthorInfo(a *models.Author) error {
	return s.db.Save(a).Error
}

func (s *ZsxqDBService) GetArticle(aid string) (*models.Article, error) {
	var article models.Article
	if err := s.db.Where("id = ?", aid).First(&article).Error; err != nil {
		return nil, err
	}
	return &article, nil
}

func (s *ZsxqDBService) GetAuthorName(aid int) (string, error) {
	var author models.Author
	if err := s.db.Where("id = ?", aid).First(&author).Error; err != nil {
		return "", err
	}
	return author.Name, nil
}

func (s *ZsxqDBService) GetAuthorID(name string) (int, error) {
	var author models.Author
	if err := s.db.Where("alias = ? or name = ?", name, name).First(&author).Error; err != nil {
		return 0, err
	}
	return author.ID, nil
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
