package parse

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"

	"go.uber.org/zap"
)

type ArticleParser interface {
	// ParseArticleList parse api response from https://www.zhihu.com/api/v4/members/{url_token}/articles
	ParseArticleList(apiResp []byte, index int, logger *zap.Logger) (paging apiModels.Paging, articlesExcerpt []apiModels.Article, articles []json.RawMessage, err error)
	// ParseArticle parse single article
	ParseArticle(content []byte, logger *zap.Logger) error
}

func (p *ParseService) ParseArticleList(apiResp []byte, index int, logger *zap.Logger) (paging apiModels.Paging, articlesExcerpt []apiModels.Article, articles []json.RawMessage, err error) {
	logger.Info("Start to parse article list", zap.Int("article_list_page_index", index))

	articleList := apiModels.ArticleList{}
	if err = json.Unmarshal(apiResp, &articleList); err != nil {
		logListPayloadDiagnostics(logger, "article", index, apiResp, err)
		return apiModels.Paging{}, nil, nil, fmt.Errorf("failed to unmarshal article list: %w", err)
	}
	logger.Info("Unmarshal article list successfully",
		zap.Int("data_count", len(articleList.Data)),
		zap.Int("paging_total", articleList.Paging.Totals),
		zap.Bool("is_end", articleList.Paging.IsEnd))

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
func (p *ParseService) ParseArticle(content []byte, logger *zap.Logger) (err error) {
	article := apiModels.Article{}
	if err = json.Unmarshal(content, &article); err != nil {
		return fmt.Errorf("failed to unmarshal article: %w", err)
	}
	logger.Info("Unmarshal article successfully")

	article.ID, err = anyToID(article.RawID)
	if err != nil {
		return fmt.Errorf("failed to convert article id from any to int: %w", err)
	}

	articleInDB, err := loadOrAbsent(p.db.GetArticle, article.ID)
	if err != nil {
		return fmt.Errorf("failed to get article from db: %w", err)
	}
	if articleInDB != nil && storedIsCurrent(articleInDB.UpdateAt, time.Unix(article.UpdateAt, 0)) {
		logger.Info("Article already up-to-date, skip re-parsing")
		return nil
	}

	// 抓取期仍下载并转存图片对象（副作用不变），但对象元数据不即时写库、随根行同事务提交；
	// 换链已移到读取期纯渲染。convertedBytes 只用于定位待下载图片；article 无 word_count/detect，
	// 不做 transient 渲染，正文一律读取期从 raw 重放（见 render 包）。
	convertedBytes, err := p.htmlToMarkdown.Convert([]byte(article.HTML))
	if err != nil {
		return fmt.Errorf("failed to convert html to markdown: %w", err)
	}
	objects, err := p.downloadImageObjects(string(convertedBytes), article.ID, common.ZhihuArticle, logger)
	if err != nil {
		return fmt.Errorf("failed to download images: %w", err)
	}
	logger.Info("Parse html successfully")

	// 原子提交：作者 + 图片对象 + article 根行同一事务，一起提交或一起回滚（plan 决策 4）；
	// 事务内根行最后写只是可读性约定，无 FK 强制、不改变回滚语义。正文读取期从 raw + 侧表重放，
	// 抓取期不持久化（已无 text 列）。article 无 word_count/detect，故无需 transient 渲染。
	if err = p.db.SaveArticleTx(&db.Article{
		ID:       article.ID,
		Title:    article.Title,
		AuthorID: article.Author.ID,
		CreateAt: time.Unix(article.CreateAt, 0),
		UpdateAt: time.Unix(article.UpdateAt, 0),
		Raw:      content,
	}, &db.Author{
		ID:   article.Author.ID,
		Name: article.Author.Name,
	}, objects); err != nil {
		return fmt.Errorf("failed to save article to db: %w", err)
	}
	logger.Info("Save article info to db successfully")

	return nil
}
