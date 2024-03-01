package parse

import (
	"encoding/json"
	"time"

	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
	"go.uber.org/zap"
)

type ArticleParser interface {
	ParseArticleList(content []byte, index int) (paging apiModels.Paging, articles []apiModels.Article, err error)
	ParseArticle(content []byte) (text string, err error)
}

func (p *ParseService) ParseArticleList(content []byte, index int) (paging apiModels.Paging, articles []apiModels.Article, err error) {
	logger := p.l.With(zap.Int("article list page", index))

	articleList := apiModels.ArticleList{}
	if err = json.Unmarshal(content, &articleList); err != nil {
		return apiModels.Paging{}, nil, err
	}
	logger.Info("unmarshal article list successfully")

	return articleList.Paging, articleList.Data, nil
}

// ParseArticle parses the zhihu.com/api/v4 resp
func (p *ParseService) ParseArticle(content []byte) (text string, err error) {
	article := apiModels.Article{}
	if err = json.Unmarshal(content, &article); err != nil {
		return "", err
	}
	logger := p.l.With(zap.Int("article_id", article.ID))
	logger.Info("unmarshal article successfully")

	text, err = p.parseHTML(article.HTML, article.ID, common.TypeZhihuArticle, logger)
	if err != nil {
		return "", err
	}
	logger.Info("parse html successfully")

	formattedText, err := p.mdfmt.FormatStr(text)
	if err != nil {
		return "", err
	}
	logger.Info("format markdown text successfully")

	if err = p.db.SaveAuthor(&db.Author{
		ID:   article.Author.ID,
		Name: article.Author.Name,
	}); err != nil {
		return "", err
	}
	logger.Info("save author to db successfully")

	if err = p.db.SaveArticle(&db.Article{
		ID:       article.ID,
		Title:    article.Title,
		Text:     formattedText,
		AuthorID: article.Author.ID,
		CreateAt: time.Unix(article.CreateAt, 0),
		Raw:      content,
	}); err != nil {
		return "", err
	}
	logger.Info("save article to db successfully")

	return formattedText, nil
}
