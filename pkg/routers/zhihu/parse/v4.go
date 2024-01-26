package parse

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
	"go.uber.org/zap"
)

func (p *V4Parser) ParseAnswer(content []byte) (err error) {
	var answer apiModels.V4Answer
	if err = json.Unmarshal(content, &answer); err != nil {
		return err
	}
	logger := p.logger.With(zap.Int("answer_id", answer.ID))
	logger.Info("unmarshal answer successfully")

	contentStr, err := p.parserContent([]byte(answer.Content), answer.ID, logger)
	if err != nil {
		return err
	}
	logger.Info("parse content successfully")

	content, err = p.mdfmt.Format([]byte(contentStr))
	if err != nil {
		return err
	}
	logger.Info("format content successfully")

	if err = p.db.SaveAuthor(&db.Author{
		ID:   answer.Author.ID,
		Name: answer.Author.Name,
	}); err != nil {
		return err
	}
	logger.Info("save author successfully")

	if err = p.db.SaveQuestion(&db.Question{
		ID:          answer.Question.ID,
		CreatedTime: time.Unix(int64(answer.Question.CreatedTime), 0),
		Title:       answer.Question.Title,
	}); err != nil {
		return err
	}
	logger.Info("save question successfully")

	if err = p.db.SaveAnswer(&db.Answer{
		ID:          answer.ID,
		QuestionID:  answer.Question.ID,
		AuthorID:    answer.Author.ID,
		CreatedTime: time.Unix(int64(answer.CreatedTime), 0),
		Text:        string(content),
		Raw: func() []byte {
			raw, _ := json.Marshal(answer)
			return raw
		}(),
		Status: db.AnswerStatusCompleted,
	}); err != nil {
		return err
	}
	logger.Info("save answer successfully")

	return nil
}

func (p *V4Parser) parserContent(content []byte, ansID int, logger *zap.Logger) (string, error) {
	result, err := p.htmlToMarkdown.Convert(content)
	if err != nil {
		return "", err
	}
	logger.Info("convert html to markdown successfully")

	text, err := p.parseImages(string(result), ansID, logger)
	if err != nil {
		return "", err
	}

	return text, nil
}

// Note: it should have been implemented in render/html.go,
// in that case we must use go routine and add a db to render.
func (p *V4Parser) parseImages(content string, ansID int, logger *zap.Logger) (result string, err error) {
	links := findImageLinks(content)
	for _, l := range links {
		logger := logger.With(zap.String("url", l))
		id := urlToID(l) // generate a unique int id from url by hash

		resp, err := p.request.NoLimitStream(l)
		if err != nil {
			return "", err
		}
		logger.Info("get image stream succussfully", zap.String("url", l))

		const zhihuImageObjectKeyLayout = "zhihu/%d.jpg"
		objectKey := fmt.Sprintf(zhihuImageObjectKeyLayout, id)
		if err = p.file.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
			return "", err
		}
		logger.Info("save image to minio successfully", zap.String("object_key", objectKey))

		if err = p.db.SaveObjectInfo(&db.Object{
			ID:              id,
			Type:            db.ObjectImageType,
			ContentType:     db.ContentTypeAnswer,
			ContentID:       ansID,
			ObjectKey:       objectKey,
			URL:             l,
			StorageProvider: []string{p.file.AssetsDomain()},
		}); err != nil {
			return "", err
		}
		logger.Info("save object info successfully")

		objectURL := fmt.Sprintf("%s/%s", p.file.AssetsDomain(), objectKey)
		content = replaceImageLinks(content, objectKey, l, objectURL)
		logger.Info("replace image link successfully", zap.String("object_url", objectURL))
	}
	return content, nil
}
