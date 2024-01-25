package parse

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
)

func (p *V4Parser) ParseAnswer(content []byte) (err error) {
	var answer apiModels.V4Answer
	if err = json.Unmarshal(content, &answer); err != nil {
		return err
	}

	content, err = p.parserContent([]byte(answer.Content), answer.ID)
	if err != nil {
		return err
	}

	if err = p.db.SaveAuthor(&db.Author{
		ID:   answer.Author.ID,
		Name: answer.Author.Name,
	}); err != nil {
		return err
	}

	if err = p.db.SaveQuestion(&db.Question{
		ID:          answer.Question.ID,
		CreatedTime: time.Unix(int64(answer.Question.CreatedTime), 0),
		Title:       answer.Question.Title,
	}); err != nil {
		return err
	}

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
	}); err != nil {
		return err
	}

	return nil
}

func (p *V4Parser) parserContent(content []byte, ansID int) ([]byte, error) {
	result, err := p.htmlToMarkdown.Convert(content)
	if err != nil {
		return nil, err
	}

	text, err := p.parseImages([]byte(result), ansID)
	if err != nil {
		return nil, err
	}

	return []byte(text), nil
}

// Note: it should have been implemented in render/html.go,
// in that case we must use go routine and add a db to render.
func (p *V4Parser) parseImages(content []byte, ansID int) (result string, err error) {
	result = string(content)
	for _, l := range findImageLinks(string(content)) {
		id := strToInt(l)

		resp, err := p.request.NoLimitStream(l)
		if err != nil {
			return "", err
		}
		const zhihuImageObjectKeyLayout = "rss/zhihu/%d.jpg"
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

		result = replaceImageLinks(result, objectKey, l, p.file.AssetsDomain()+objectKey)
	}
	return "", nil
}
