package db

import (
	"time"

	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db/models"
	"gorm.io/gorm"
)

type DataBaseIface interface {
	// Save topic to zsxq_topic table
	SaveTopic(t *models.Topic) error
	// Get latest topic time from zsxq_topic table
	GetLatestTopicTime(tid int) (t time.Time, err error)
	// Get earliest topic time from zsxq_topic table
	GetEarliestTopicTime(tid int) (t time.Time, err error)
	// Get latest n topics from zsxq_topic table
	GetLatestNTopics(gid int, n int) (ts []models.Topic, err error)
	// Fetch n topics before time from zsxq_topic table
	FetchNTopicsBeforeTime(gid int, n int, t time.Time) (ts []models.Topic, err error)

	// Save article to zsxq_article table
	SaveArticle(a *models.Article) error
	// Get article text by id from zsxq_article table
	GetArticleText(aid string) (text string, err error)

	// Save object info to zsxq_object table
	SaveObjectInfo(o *models.Object) error
	// Get object info from zsxq_object table
	GetObjectInfo(oid int) (o *models.Object, err error)

	// Save author info to zsxq_author table
	SaveAuthorInfo(a *models.Author) error
	// Get author name by id from zsxq_author table
	GetAuthorName(aid int) (name string, err error)

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

func (s *ZsxqDBService) GetAuthorName(aid int) (string, error) {
	var author models.Author
	if err := s.db.Where("id = ?", aid).First(&author).Error; err != nil {
		return "", err
	}
	return author.Name, nil
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
