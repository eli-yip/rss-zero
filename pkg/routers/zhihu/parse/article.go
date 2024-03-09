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
	// ParseArticleList parse api response from https://www.zhihu.com/api/v4/members/{url_token}/articles
	ParseArticleList(apiResp []byte, index int) (paging apiModels.Paging, articles []apiModels.Article, err error)
	// ParseArticle parse single article
	ParseArticle(content []byte) (text string, err error)
}

func (p *ParseService) ParseArticleList(apiResp []byte, index int) (paging apiModels.Paging, articles []apiModels.Article, err error) {
	logger := p.logger.With(zap.Int("article list page", index))

	articleList := apiModels.ArticleList{}
	if err = json.Unmarshal(apiResp, &articleList); err != nil {
		logger.Info("Fail to unmarshal api response")
		return apiModels.Paging{}, nil, err
	}
	logger.Info("unmarshal article list successfully")

	return articleList.Paging, articleList.Data, nil
}

// ParseArticle parses the zhihu.com/api/v4 resp
func (p *ParseService) ParseArticle(content []byte) (text string, err error) {
	article := apiModels.Article{}
	if err = json.Unmarshal(content, &article); err != nil {
		p.logger.Info("Fail to parse api response into single article", zap.Error(err))
		return emptyString, err
	}
	logger := p.logger.With(zap.Int("article_id", article.ID))
	logger.Info("unmarshal article successfully")

	text, err = p.parseHTML(article.HTML, article.ID, common.TypeZhihuArticle, logger)
	if err != nil {
		logger.Error("Fail to parse article html", zap.Error(err))
		return emptyString, err
	}
	logger.Info("parse html successfully")

	formattedText, err := p.mdfmt.FormatStr(text)
	if err != nil {
		logger.Error("Fail to format article text", zap.Error(err))
		return emptyString, err
	}
	logger.Info("format markdown text successfully")

	if err = p.db.SaveAuthor(&db.Author{
		ID:   article.Author.ID,
		Name: article.Author.Name,
	}); err != nil {
		logger.Error("Fail to save author info to database", zap.Error(err))
		return emptyString, err
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
		logger.Error("Fail to save article to database", zap.Error(err))
		return emptyString, err
	}
	logger.Info("save article to db successfully")

	return formattedText, nil
}
