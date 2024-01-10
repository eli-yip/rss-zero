package render

import (
	"time"

	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/parse/models"
)

type Topic struct {
	// TopicID int
	Type string
	// CreateTime time.Time
	// GroupName  string
	// GroupID    int
	Talk       *models.Talk
	Question   *models.Question
	Answer     *models.Answer
	AuthorName string
	// Title      *string
	// ShareLink  string
}

type RSSTopic struct {
	TopicID    int
	GroupName  string
	GroupID    int
	Title      *string
	AuthorName string
	ShareLink  string
	CreateTime time.Time
	Text       string
}
