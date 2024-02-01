package parse

import (
	"encoding/json"
	"time"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
	"go.uber.org/zap"
)

// ParseAnswer receives api.zhihu.com resp and parse it
func (p *Parser) ParseAnswer(content []byte) (err error) {
	answer := apiModels.Answer{}
	if err = json.Unmarshal(content, &answer); err != nil {
		return err
	}
	logger := p.logger.With(zap.Int("answer_id", answer.ID))
	logger.Info("unmarshal answer successfully")

	text, err := p.parseHTML(answer.HTML, answer.ID, logger)
	if err != nil {
		return err
	}
	logger.Info("parse html content successfully")

	formattedText, err := p.mdfmt.FormatStr(text)
	if err != nil {
		return err
	}
	logger.Info("format markdown content successfully")

	if err = p.db.SaveAuthor(&db.Author{
		ID:   answer.Author.ID,
		Name: answer.Author.Name,
	}); err != nil {
		return err
	}
	logger.Info("save author to db successfully")

	if err = p.db.SaveQuestion(&db.Question{
		ID:       answer.Question.ID,
		CreateAt: time.Unix(answer.Question.CreateAt, 0),
		Title:    answer.Question.Title,
	}); err != nil {
		return err
	}
	logger.Info("save question to db successfully")

	if err = p.db.SaveAnswer(&db.Answer{
		ID:         answer.ID,
		QuestionID: answer.Question.ID,
		AuthorID:   answer.Author.ID,
		CreateAt:   time.Unix(answer.CreateAt, 0),
		Text:       formattedText,
		Raw:        content,
		Status:     db.AnswerStatusCompleted,
	}); err != nil {
		return err
	}
	logger.Info("save answer to db successfully")

	return nil
}
