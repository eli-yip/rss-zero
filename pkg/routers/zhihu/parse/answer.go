package parse

import (
	"encoding/json"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
)

type AnswerParser interface {
	ParseAnswerList(content []byte, index int, logger *zap.Logger) (paging apiModels.Paging, answers []apiModels.Answer, err error)
	ParseAnswer(content []byte, authorID string, logger *zap.Logger) (text string, err error)
}

func (p *ParseService) ParseAnswerList(content []byte, index int, logger *zap.Logger) (paging apiModels.Paging, answers []apiModels.Answer, err error) {
	logger.Info("Start to parse answer list", zap.Int("answer_list_page_index", index))

	answerList := apiModels.AnswerList{}
	if err = json.Unmarshal(content, &answerList); err != nil {
		return apiModels.Paging{}, nil, fmt.Errorf("failed to unmarshal answer list: %w", err)
	}
	logger.Info("Unmarshal answer list successfully")

	return answerList.Paging, answerList.Data, nil
}

// ParseAnswer receives api.zhihu.com resp and parse it
func (p *ParseService) ParseAnswer(content []byte, authorID string, logger *zap.Logger) (text string, err error) {
	answer := apiModels.Answer{}
	if err = json.Unmarshal(content, &answer); err != nil {
		return emptyString, fmt.Errorf("failed to unmarshal answer: %w", err)
	}
	logger.Info("Unmarshal answer successfully")

	text, err = p.parseHTML(answer.HTML, answer.ID, common.TypeZhihuAnswer)
	if err != nil {
		return emptyString, fmt.Errorf("failed to parse html content: %w", err)
	}
	logger.Info("Parse html content successfully")

	formattedText, err := p.mdfmt.FormatStr(text)
	if err != nil {
		return emptyString, fmt.Errorf("failed to format markdown content: %w", err)
	}
	logger.Info("Format markdown content successfully")

	if err = p.db.SaveQuestion(&db.Question{
		ID:       answer.Question.ID,
		CreateAt: time.Unix(answer.Question.CreateAt, 0),
		Title:    answer.Question.Title,
	}); err != nil {
		return emptyString, fmt.Errorf("failed to save question info to db: %w", err)
	}
	logger.Info("Save question info to db successfully", zap.String("question_title", answer.Question.Title))

	if err = p.db.SaveAnswer(&db.Answer{
		ID:         answer.ID,
		QuestionID: answer.Question.ID,
		AuthorID:   authorID,
		CreateAt:   time.Unix(answer.CreateAt, 0),
		Text:       formattedText,
		Raw:        content, // NOTE: see db.Answer.Raw comment
		Status:     db.AnswerStatusCompleted,
		WordCount:  md.Count(text),
	}); err != nil {
		return emptyString, fmt.Errorf("failed to save answer to db: %w", err)
	}
	logger.Info("Save answer info to db successfully")

	return formattedText, nil
}
