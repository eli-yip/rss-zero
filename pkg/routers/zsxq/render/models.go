package render

import (
	"time"

	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
)

// Topic is the struct used to render a topic
//
// Not every field is used in every render method.
//
// e.g.: Time, Digested, Title, Text is only used
// in ToFullText method.
type Topic struct {
	ID         int // Used for id trace
	GroupID    int
	Time       time.Time
	Type       string
	Digested   bool
	Talk       *models.Talk
	Question   *models.Question
	Answer     *models.Answer
	Title      *string
	Text       string
	AuthorName string
}

type RSSTopic struct {
	TopicID    int
	GroupName  string
	GroupID    int
	Title      *string
	AuthorName string
	CreateTime time.Time
	Text       string
}
