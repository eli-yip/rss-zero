package parse

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
	"go.uber.org/zap"
)

func (p *Parser) SplitPosts(content []byte) (posts []apiModels.Article, err error) {
	initialData, err := p.getInitialData(content)
	if err != nil {
		return nil, err
	}
	p.logger.Info("get initial data successfully")

	var htmlPost apiModels.HTMLPost
	if err = json.Unmarshal([]byte(initialData), &htmlPost); err != nil {
		return nil, err
	}
	p.logger.Info("unmarshal initial data successfully")

	for _, a := range htmlPost.InitialState.Entities.Articles {
		posts = append(posts, a)
	}

	return posts, nil
}

func (p *Parser) ParsePosts(a apiModels.Article) (err error) {
	var content []byte
	logger := p.logger.With(zap.Int("post_id", a.ID))

	text, err := p.parseContent([]byte(a.Content), a.ID, logger)
	if err != nil {
		return err
	}
	logger.Info("parse content successfully")

	content, err = p.mdfmt.Format([]byte(text))
	if err != nil {
		return err
	}
	logger.Info("format content successfully")

	if err = p.db.SaveAuthor(&db.Author{
		ID:   a.Author.ID,
		Name: a.Author.Name,
	}); err != nil {
		return err
	}
	logger.Info("save author successfully")

	if err = p.db.SavePost(&db.Post{
		ID:          a.ID,
		Title:       a.Title,
		Text:        string(content),
		AuthorID:    a.Author.ID,
		CreatedTime: time.Unix(int64(a.CreatedTime), 0),
		Raw:         func() []byte { b, _ := json.Marshal(a); return b }(),
	}); err != nil {
		return err
	}
	logger.Info("save post successfully")

	return nil
}

func (p *Parser) getInitialData(content []byte) (initialData string, err error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(content))
	if err != nil {
		return "", err
	}

	return doc.Find("body script#js-initialData").Text(), nil
}
