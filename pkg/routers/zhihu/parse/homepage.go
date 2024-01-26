package parse

import (
	"encoding/json"
	"time"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
)

type HomepageParser struct{ db db.DB }

func NewHomepageParser(db db.DB) *HomepageParser {
	return &HomepageParser{db: db}
}

func (p *HomepageParser) ParseHomepage(content []byte) (
	isEnd bool, totals int, next string, err error) {
	var feed apiModels.HomepageFeed
	if err = json.Unmarshal(content, &feed); err != nil {
		return false, 0, "", err
	}

	for _, a := range feed.Data {
		if err := p.db.SaveAnswer(&db.Answer{
			ID:          int(a.ID),
			QuestionID:  a.Question.ID,
			CreatedTime: time.Unix(int64(a.CreatedTime), 0),
		}); err != nil {
			return false, 0, "", err
		}
	}

	return feed.Paging.IsEnd, feed.Paging.Totals, feed.Paging.Next, nil
}
