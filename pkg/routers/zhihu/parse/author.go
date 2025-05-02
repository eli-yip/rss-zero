package parse

import (
	"fmt"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"go.uber.org/zap"
)

type AuthorParser interface {
	// Parse result from api.zhihu.com/people/{url_token}
	ParseAuthorName(apiResp []byte, logger *zap.Logger) (authorName string, err error)
}

func (p *ParseService) ParseAuthorName(apiResp []byte, logger *zap.Logger) (authorName string, err error) {
	_, answers, _, err := p.ParseAnswerList(apiResp, 0, logger)
	if err != nil {
		return "", fmt.Errorf("failed to parse answer list: %w", err)
	}
	if len(answers) == 0 {
		return "", fmt.Errorf("empty answer list")
	}

	author := answers[0].Author
	if err = p.db.SaveAuthor(&db.Author{
		ID:   author.ID,
		Name: author.Name,
	}); err != nil {
		return emptyString, fmt.Errorf("failed to save author info %s %s to db: %w", author.ID, author.Name, err)
	}

	return author.Name, nil
}
