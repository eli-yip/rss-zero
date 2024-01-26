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

func (p *V4Parser) ParsePosts(content []byte) (err error) {
	initialData, err := p.getInitialData(content)
	if err != nil {
		return err
	}
	p.logger.Info("get initial data successfully")

	var htmlPost apiModels.HTMLPost
	if err = json.Unmarshal([]byte(initialData), &htmlPost); err != nil {
		return err
	}
	p.logger.Info("unmarshal initial data successfully")

	for _, a := range htmlPost.InitialState.Entities.Articles {
		logger := p.logger.With(zap.Int("post_id", a.ID))

		text, err := p.parserContent([]byte(a.Content), a.ID, logger)
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
			Content:     string(content),
			AuthorID:    a.Author.ID,
			CreatedTime: time.Unix(int64(a.CreatedTime), 0),
			Raw:         func() []byte { b, _ := json.Marshal(a); return b }(),
		}); err != nil {
			return err
		}
		logger.Info("save post successfully")
	}

	return nil
}

func (p *V4Parser) getInitialData(content []byte) (initialData string, err error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(content))
	if err != nil {
		return "", err
	}

	return doc.Find("body script#js-initialData").Text(), nil
}
