package parse

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
)

type AnswerParser interface {
	// answerExcerpt: answer list for answer id etc. why: some may need some info before parse raw message
	ParseAnswerList(content []byte, index int, logger *zap.Logger) (paging apiModels.Paging, answersExcerpt []apiModels.Answer, answers []json.RawMessage, err error)
	ParseAnswer(content []byte, authorID string, logger *zap.Logger) (text string, err error)
}

func (p *ParseService) ParseAnswerList(content []byte, index int, logger *zap.Logger) (paging apiModels.Paging, answersExcerpt []apiModels.Answer, answers []json.RawMessage, err error) {
	logger.Info("Start to parse answer list", zap.Int("answer_list_page_index", index))

	answerList := apiModels.AnswerList{}
	if err = json.Unmarshal(content, &answerList); err != nil {
		return apiModels.Paging{}, nil, nil, fmt.Errorf("failed to unmarshal answer list: %w", err)
	}
	logger.Info("Unmarshal answer list successfully")

	for _, rawMessage := range answerList.Data {
		answer := apiModels.Answer{}
		if err = json.Unmarshal(rawMessage, &answer); err != nil {
			return apiModels.Paging{}, nil, nil, fmt.Errorf("failed to unmarshal answer: %w, data: %s", err, string(rawMessage))
		}

		if f, ok := answer.RawID.(float64); ok {
			answer.ID = int(f)
			if answer.ID < 1000 {
				logger.Warn("Answer id is float64, may cause some issue", zap.Int("new_answer_id", answer.ID), zap.Float64("old_answer_id", f))
				return apiModels.Paging{}, nil, nil, errors.New("skip this sub")
			}
		} else if s, ok := answer.RawID.(string); ok {
			answer.ID, err = strconv.Atoi(s)
			if err != nil {
				return apiModels.Paging{}, nil, nil, fmt.Errorf("failed to convert answer id from string to int: %w, id: %s", err, s)
			}
			if answer.ID < 1000 {
				logger.Warn("Answer id is string, may cause some issue", zap.Int("new_answer_id", answer.ID), zap.String("old_answer_id", s))
				return apiModels.Paging{}, nil, nil, errors.New("skip this sub")
			}
		} else {
			return apiModels.Paging{}, nil, nil, fmt.Errorf("failed to convert answer id from any to int, data: %s", string(rawMessage))
		}

		if f, ok := answer.Question.RawID.(float64); ok {
			answer.Question.ID = int(f)
			if answer.Question.ID < 1000 {
				logger.Warn("Question id is float64, may cause some issue", zap.Int("new_question_id", answer.Question.ID), zap.Float64("old_question_id", f))
				return apiModels.Paging{}, nil, nil, errors.New("skip this sub")
			}
		} else if s, ok := answer.Question.RawID.(string); ok {
			answer.Question.ID, err = strconv.Atoi(s)
			if err != nil {
				return apiModels.Paging{}, nil, nil, fmt.Errorf("failed to convert question id from string to int: %w, id: %s", err, s)
			}
			if answer.Question.ID < 1000 {
				logger.Warn("Question id is string, may cause some issue", zap.Int("new_question_id", answer.Question.ID), zap.String("old_question_id", s))
				return apiModels.Paging{}, nil, nil, errors.New("skip this sub")
			}
		} else {
			return apiModels.Paging{}, nil, nil, fmt.Errorf("failed to convert question id from any to int, data: %s", string(rawMessage))
		}

		answersExcerpt = append(answersExcerpt, answer)
	}

	return answerList.Paging, answersExcerpt, answerList.Data, nil
}

func (p *ParseService) ParseAnswer(content []byte, authorID string, logger *zap.Logger) (text string, err error) {
	answer := apiModels.Answer{}
	if err = json.Unmarshal(content, &answer); err != nil {
		return emptyString, fmt.Errorf("failed to unmarshal answer: %w", err)
	}
	logger.Info("Unmarshal answer successfully")

	answerInDB, exist, err := checkAnswerExist(answer.ID, p.db)
	if err != nil {
		return emptyString, fmt.Errorf("failed to check answer exist: %w", err)
	}
	if exist {
		if answerInDB.UpdateAt.IsZero() {
			logger.Info("Answer already exist, updated_at is zero, skip this answer")
			return answerInDB.Text, nil
		}
		answerUpdateAt := time.Unix(answer.UpdateAt, 0)
		if answerUpdateAt.After(answerInDB.UpdateAt) {
			logger.Info("Answer already exist, updated_at is newer, update this answer")
		} else {
			logger.Info("Answer already exist, updated_at is older, skip this answer")
			return answerInDB.Text, nil
		}
	}

	text, err = p.parseHTML(answer.HTML, answer.ID, common.TypeZhihuAnswer, logger)
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
		UpdateAt:   time.Unix(answer.UpdateAt, 0),
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

func checkAnswerExist(answerID int, db db.DB) (answer *db.Answer, exist bool, err error) {
	answer, err = db.GetAnswer(answerID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to get answer from db: %w", err)
	}
	return answer, true, nil
}
