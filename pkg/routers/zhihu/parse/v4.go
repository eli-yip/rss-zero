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

	contentStr, err := p.parserContent([]byte(answer.Content), answer.ID)
	if err != nil {
		return err
	}
	logger.Info("parse content successfully")

	if err = p.db.SaveAuthor(&db.Author{
		ID:   answer.Author.ID,
		Name: answer.Author.Name,
	}); err != nil {
		return err
	}
	logger.Info("save author successfully",
		zap.String("author_id", answer.Author.ID),
		zap.String("author_name", answer.Author.Name))

	if err = p.db.SaveQuestion(&db.Question{
		ID:          answer.Question.ID,
		CreatedTime: time.Unix(int64(answer.Question.CreatedTime), 0),
		Title:       answer.Question.Title,
	}); err != nil {
		return err
	}
	logger.Info("save question successfully",
		zap.Int("question_id", answer.Question.ID),
		zap.String("question_title", answer.Question.Title))

	if err = p.db.SaveAnswer(&db.Answer{
		ID:          answer.ID,
		QuestionID:  answer.Question.ID,
		AuthorID:    answer.Author.ID,
		CreatedTime: time.Unix(int64(answer.CreatedTime), 0),
		Text:        contentStr,
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

func (p *V4Parser) parserContent(content []byte, ansID int) (string, error) {
	result, err := p.htmlToMarkdown.Convert(content)
	if err != nil {
		return "", err
	}

	text, err := p.parseImages(string(result), ansID)
	if err != nil {
		return "", err
	}

	return text, nil
}

// Note: it should have been implemented in render/html.go,
// in that case we must use go routine and add a db to render.
func (p *V4Parser) parseImages(content string, ansID int) (result string, err error) {
	links := findImageLinks(content)
	for _, l := range links {
		id := strToInt(l)

		resp, err := p.request.NoLimitStream(l)
		if err != nil {
			return "", err
		}
		const zhihuImageObjectKeyLayout = "zhihu/%d.jpg"
		objectKey := fmt.Sprintf(zhihuImageObjectKeyLayout, id)
		if err = p.file.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
			return "", err
		}

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

		objectURL := fmt.Sprintf("%s/%s", p.file.AssetsDomain(), objectKey)
		content = replaceImageLinks(content, objectKey, l, objectURL)
	}
	return content, nil
}
