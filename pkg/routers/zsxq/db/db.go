package db

import (
	"time"

	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db/models"
)

type DataBaseIface interface {
	DataBaseTopic
	DataBaseArticle
	DataBaseObject
	DataBaseAuthor
	DatabaseGroup
}

type DataBaseTopic interface {
	// Save topic to zsxq_topic table
	SaveTopic(t *models.Topic) error
	// Get latest topic time from zsxq_topic table
	GetLatestTopicTime(tid int) (t time.Time, err error)
	// Get earliest topic time from zsxq_topic table
	GetEarliestTopicTime(tid int) (t time.Time, err error)
	// Get latest n topics from zsxq_topic table
	GetLatestNTopics(gid int, n int) (ts []models.Topic, err error)
	// Get All ids from zsxq_topic table
	GetAllTopicIDs(gid int) (ids []int, err error)
	// Fetch n topics before time from zsxq_topic table
	FetchNTopicsBeforeTime(gid int, n int, t time.Time) (ts []models.Topic, err error)
	// Fetch n topics with options from zsxq_topic table
	FetchNTopics(n int, opt Options) (ts []models.Topic, err error)
}

type DataBaseArticle interface {
	// Save article to zsxq_article table
	SaveArticle(a *models.Article) error
	// Get article
	GetArticle(aid string) (a *models.Article, err error)
	// Get article text by id from zsxq_article table
	GetArticleText(aid string) (text string, err error)
}

type DataBaseObject interface {
	// Save object info to zsxq_object table
	SaveObjectInfo(o *models.Object) error
	// Get object info from zsxq_object table
	GetObjectInfo(oid int) (o *models.Object, err error)
}

type DataBaseAuthor interface {
	// Save author info to zsxq_author table
	SaveAuthorInfo(a *models.Author) error
	// Get author name by id from zsxq_author table
	GetAuthorName(aid int) (name string, err error)
	// Get author id by name or alias from zsxq_author table
	GetAuthorID(name string) (id int, err error)
}

type DatabaseGroup interface {
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
