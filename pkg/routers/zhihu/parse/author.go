package parse

import (
	"encoding/json"
	"fmt"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"

	"go.uber.org/zap"
)

type AuthorParser interface {
	// Parse result from api.zhihu.com/people/{url_token}
	ParseAuthorName(apiResp []byte) (authorName string, err error)
}

func (p *ParseService) ParseAuthorName(apiResp []byte) (authorName string, err error) {
	var author apiModels.Author
	if err = json.Unmarshal(apiResp, &author); err != nil {
		p.logger.Error("Fail to unmarshal api response", zap.Error(err))
		return emptyString, fmt.Errorf("failed to unmarshal api response: %w", err)
	}

	if err = p.db.SaveAuthor(&db.Author{
		ID:   author.ID,
		Name: author.Name,
	}); err != nil {
		return emptyString, fmt.Errorf("failed to save author info %s %s to db: %w", author.ID, author.Name, err)
	}

	return author.Name, nil
}
