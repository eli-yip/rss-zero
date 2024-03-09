package parse

import (
	"encoding/json"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"

	"go.uber.org/zap"
)

type AuthorParser interface {
	// Parse result from api.zhihu.com/people/{url_token}
	ParseAuthorName(apiResp []byte) (authorName string, err error)
}

func (p *ParseService) ParseAuthorName(apiResp []byte) (authorName string, err error) {
	p.logger.Info("start to parse author name")

	var author apiModels.Author
	if err = json.Unmarshal(apiResp, &author); err != nil {
		p.logger.Error("Fail to unmarshal api response", zap.Error(err))
		return emptyString, err
	}
	p.logger.Info("Parsed api response")

	if err = p.db.SaveAuthor(&db.Author{
		ID:   author.ID,
		Name: author.Name,
	}); err != nil {
		p.logger.Error("Fail to save parsed author name to db", zap.String("author_id", author.ID), zap.String("author_name", author.Name))
		return emptyString, err
	}

	return author.Name, nil
}
