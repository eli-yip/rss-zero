package parse

import (
	"encoding/json"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
)

type AuthorParser interface {
	// Parse result from api.zhihu.com/people/{url_token}
	ParseAuthorName(content []byte) (authorName string, err error)
}

func (p *ParseService) ParseAuthorName(content []byte) (authorName string, err error) {
	p.l.Info("start to parse author name")

	var author apiModels.Author
	if err = json.Unmarshal(content, &author); err != nil {
		return "", err
	}

	if err = p.db.SaveAuthor(&db.Author{
		ID:   author.ID,
		Name: author.Name,
	}); err != nil {
		return "", err
	}

	return author.Name, nil
}
