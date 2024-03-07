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

type AnswerParser interface {
	ParseAnswerList(content []byte, index int) (paging apiModels.Paging, answers []apiModels.Answer, err error)
	ParseAnswer(content []byte) (text string, err error)
}

func (p *ParseService) ParseAnswerList(content []byte, index int) (paging apiModels.Paging, answers []apiModels.Answer, err error) {
	logger := p.l.With(zap.Int("answer list page index", index))

	answerList := apiModels.AnswerList{}
	if err = json.Unmarshal(content, &answerList); err != nil {
		return apiModels.Paging{}, nil, err
	}
	logger.Info("unmarshal answer list successfully")

	return answerList.Paging, answerList.Data, nil
}

// ParseAnswer receives api.zhihu.com resp and parse it
func (p *ParseService) ParseAnswer(content []byte) (text string, err error) {
	answer := apiModels.Answer{}
	if err = json.Unmarshal(content, &answer); err != nil {
		return "", err
	}
	logger := p.l.With(zap.Int("answer_id", answer.ID))
	logger.Info("unmarshal answer successfully")

	text, err = p.parseHTML(answer.HTML, answer.ID, common.TypeZhihuAnswer, logger)
	if err != nil {
		return "", err
	}
	logger.Info("parse html content successfully")

	formattedText, err := p.mdfmt.FormatStr(text)
	if err != nil {
		return "", err
	}
	logger.Info("format markdown content successfully")

	if err = p.db.SaveAuthor(&db.Author{
		ID:   answer.Author.ID,
		Name: answer.Author.Name,
	}); err != nil {
		return "", fmt.Errorf("fail to save author to db: %w, author_id: %s, author_name: %s", err, answer.Author.ID, answer.Author.Name)
	}
	logger.Info("save author to db successfully")

	if err = p.db.SaveQuestion(&db.Question{
		ID:       answer.Question.ID,
		CreateAt: time.Unix(answer.Question.CreateAt, 0),
		Title:    answer.Question.Title,
	}); err != nil {
		return "", err
	}
	logger.Info("save question to db successfully")

	if err = p.db.SaveAnswer(&db.Answer{
		ID:         answer.ID,
		QuestionID: answer.Question.ID,
		AuthorID:   answer.Author.ID,
		CreateAt:   time.Unix(answer.CreateAt, 0),
		Text:       formattedText,
		Raw:        content, // NOTE: see db.Answer.Raw comment
		Status:     db.AnswerStatusCompleted,
	}); err != nil {
		return "", err
	}
	logger.Info("save answer to db successfully")

	return formattedText, nil
}
