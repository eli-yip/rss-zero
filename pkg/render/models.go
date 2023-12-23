package render

import (
	"time"

	"github.com/eli-yip/zsxq-parser/pkg/parse/models"
)

type Topic struct {
	TopicID    int
	Type       string
	CreateTime time.Time
	GroupName  string
	GroupID    int
	Talk       *models.Talk
	Question   *models.Question
	Answer     *models.Answer
	Author     string
	Title      *string
	ShareLink  string
}

type RSSTopic struct {
	TopicID    int
	GroupName  string
	GroupID    int
	Title      *string
	Author     string
	ShareLink  string
	CreateTime time.Time
	Text       string
}
