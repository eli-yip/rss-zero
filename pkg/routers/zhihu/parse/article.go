package parse

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
	"gorm.io/gorm"

	"go.uber.org/zap"
)

type ArticleParser interface {
	// ParseArticleList parse api response from https://www.zhihu.com/api/v4/members/{url_token}/articles
	ParseArticleList(apiResp []byte, index int, logger *zap.Logger) (paging apiModels.Paging, articlesExcerpt []apiModels.Article, articles []json.RawMessage, err error)
	// ParseArticle parse single article
	ParseArticle(content []byte, logger *zap.Logger) (text string, err error)
}

func (p *ParseService) ParseArticleList(apiResp []byte, index int, logger *zap.Logger) (paging apiModels.Paging, articlesExcerpt []apiModels.Article, articles []json.RawMessage, err error) {
	logger.Info("Start to parse article list", zap.Int("article_list_page_index", index))

	articleList := apiModels.ArticleList{}
	if err = json.Unmarshal(apiResp, &articleList); err != nil {
		return apiModels.Paging{}, nil, nil, fmt.Errorf("failed to unmarshal article list: %w", err)
	}
	logger.Info("Unmarshal article list successfully")

	for _, rawMessage := range articleList.Data {
		article := apiModels.Article{}
		if err = json.Unmarshal(rawMessage, &article); err != nil {
			return apiModels.Paging{}, nil, nil, fmt.Errorf("failed to unmarshal article: %w, data: %s", err, string(rawMessage))
		}

		article.ID, err = anyToID(article.RawID)
		if err != nil {
			return apiModels.Paging{}, nil, nil, fmt.Errorf("failed to convert article id from any to int: %w, data: %s", err, string(rawMessage))
		}

		articlesExcerpt = append(articlesExcerpt, article)
	}

	return articleList.Paging, articlesExcerpt, articleList.Data, nil
}

// ParseArticle parses the zhihu.com/api/v4 resp
func (p *ParseService) ParseArticle(content []byte, logger *zap.Logger) (text string, err error) {
	article := apiModels.Article{}
	if err = json.Unmarshal(content, &article); err != nil {
		return emptyString, fmt.Errorf("failed to unmarshal article: %w", err)
	}
	logger.Info("Unmarshal article successfully")

	article.ID, err = anyToID(article.RawID)
	if err != nil {
		return emptyString, fmt.Errorf("failed to convert article id from any to int: %w", err)
	}

	articleInDB, exist, err := checkArticleExist(article.ID, p.db)
	if err != nil {
		return emptyString, fmt.Errorf("failed to check article exist: %w", err)
	}
	if exist {
		if articleInDB.UpdateAt.IsZero() {
			logger.Info("Article already exist, updated_at is zero, skip this article")
			return articleInDB.Text, nil
		}
		articleUpdateAt := time.Unix(article.UpdateAt, 0)
		if articleUpdateAt.After(articleInDB.UpdateAt) {
			logger.Info("Article already exist, but updated_at is newer, re-parse it")
		} else {
			logger.Info("Article already exist, skip")
			return articleInDB.Text, nil
		}
	}

	text, err = p.parseHTML(article.HTML, article.ID, common.TypeZhihuArticle, logger)
	if err != nil {
		return emptyString, fmt.Errorf("failed to parse html content: %w", err)
	}
	logger.Info("Parse html successfully")

	formattedText, err := p.mdfmt.FormatStr(text)
	if err != nil {
		return emptyString, fmt.Errorf("failed to format markdown text: %w", err)
	}
	logger.Info("Format markdown text successfully")

	if err = p.db.SaveAuthor(&db.Author{
		ID:   article.Author.ID,
		Name: article.Author.Name,
	}); err != nil {
		return emptyString, fmt.Errorf("failed to save author info to db: %w", err)
	}
	logger.Info("Save author info to db successfully")

	if err = p.db.SaveArticle(&db.Article{
		ID:       article.ID,
		Title:    article.Title,
		Text:     formattedText,
		AuthorID: article.Author.ID,
		CreateAt: time.Unix(article.CreateAt, 0),
		UpdateAt: time.Unix(article.UpdateAt, 0),
		Raw:      content,
	}); err != nil {
		return emptyString, fmt.Errorf("failed to save article to db: %w", err)
	}
	logger.Info("Save article info to db successfully")

	return formattedText, nil
}

func checkArticleExist(articleID int, db db.DB) (article *db.Article, exist bool, err error) {
	article, err = db.GetArticle(articleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to get article from db: %w", err)
	}
	return article, true, nil
}
