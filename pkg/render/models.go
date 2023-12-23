package render

import (
	"time"

	"github.com/eli-yip/zsxq-parser/pkg/parse/models"
)

type Topic struct {
	TopicID    int
	GroupName  string
	Type       string
	CreateTime time.Time
	Talk       *models.Talk
	Question   *models.Question
	Answer     *models.Answer
	Title      *string
	Author     string
	ShareLink  string
}
