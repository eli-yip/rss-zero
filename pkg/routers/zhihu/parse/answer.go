package parse

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
)

const (
	answerTypePaidColumnContent = "paid_column_content"
	paidColumnContentNotice     = "**该文章为付费专栏内容**"
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
		logListPayloadDiagnostics(logger, "answer", index, content, err)
		return apiModels.Paging{}, nil, nil, fmt.Errorf("failed to unmarshal answer list: %w", err)
	}
	logger.Info("Unmarshal answer list successfully",
		zap.Int("data_count", len(answerList.Data)),
		zap.Int("paging_total", answerList.Paging.Totals),
		zap.Bool("is_end", answerList.Paging.IsEnd))

	for _, rawMessage := range answerList.Data {
		answer := apiModels.Answer{}
		if err = json.Unmarshal(rawMessage, &answer); err != nil {
			return apiModels.Paging{}, nil, nil, fmt.Errorf("failed to unmarshal answer: %w, data: %s", err, string(rawMessage))
		}

		answer.ID, err = anyToID(answer.RawID)
		if err != nil {
			return apiModels.Paging{}, nil, nil, fmt.Errorf("failed to convert answer id from any to int: %w, data: %s", err, string(rawMessage))
		}
		answer.Question.ID, err = anyToID(answer.Question.RawID)
		if err != nil {
			return apiModels.Paging{}, nil, nil, fmt.Errorf("failed to convert question id from any to int: %w, data: %s", err, string(rawMessage))
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

	answer.ID, err = anyToID(answer.RawID)
	if err != nil {
		return emptyString, fmt.Errorf("failed to convert answer id from any to int: %w", err)
	}
	answer.Question.ID, err = anyToID(answer.Question.RawID)
	if err != nil {
		return emptyString, fmt.Errorf("failed to convert question id from any to int: %w", err)
	}

	answerInDB, err := loadOrAbsent(p.db.GetAnswer, answer.ID)
	if err != nil {
		return emptyString, fmt.Errorf("failed to get answer from db: %w", err)
	}
	if answerInDB != nil && storedIsCurrent(answerInDB.UpdateAt, time.Unix(answer.UpdateAt, 0)) {
		logger.Info("Answer already up-to-date, skip re-parsing")
		return answerInDB.Text, nil
	}

	text, err = p.parseHTML(answer.HTML, answer.ID, common.ZhihuAnswer, logger)
	if err != nil {
		return emptyString, fmt.Errorf("failed to parse html content: %w", err)
	}
	text = AddPaidColumnContentNotice(text, answer.AnswerType)
	logger.Info("Parse html content successfully")

	formattedText, err := p.mdfmt.FormatStr(text)
	if err != nil {
		return emptyString, fmt.Errorf("failed to format markdown content: %w", err)
	}
	logger.Info("Format markdown content successfully")

	detectStatus := db.DetectStatusNone
	detectReason := ""
	if p.detector != nil {
		res, detected, derr := p.detector.Detect(authorID, formattedText)
		switch {
		case !detected:
			// author not registered for detection; leave DetectStatusNone
		case derr != nil:
			logger.Error("Content detect failed, fail-open", zap.Error(derr))
			detectStatus = db.DetectStatusFailed
		case res.Skip:
			detectStatus = db.DetectStatusSkipped
			detectReason = res.Reason
			logger.Info("Answer hit content detection, will be hidden from rss", zap.String("reason", res.Reason))
		default:
			detectStatus = db.DetectStatusPassed
		}
	}

	if err = p.db.SaveQuestion(&db.Question{
		ID:       answer.Question.ID,
		CreateAt: time.Unix(answer.Question.CreateAt, 0),
		Title:    answer.Question.Title,
	}); err != nil {
		return emptyString, fmt.Errorf("failed to save question info to db: %w", err)
	}
	logger.Info("Save question info to db successfully", zap.String("question_title", answer.Question.Title))

	if err = p.db.SaveAnswer(&db.Answer{
		ID:           answer.ID,
		QuestionID:   answer.Question.ID,
		AuthorID:     authorID,
		CreateAt:     time.Unix(answer.CreateAt, 0),
		UpdateAt:     time.Unix(answer.UpdateAt, 0),
		Text:         formattedText,
		Raw:          content, // NOTE: see db.Answer.Raw comment
		Status:       db.AnswerStatusCompleted,
		WordCount:    md.Count(text),
		DetectStatus: detectStatus,
		DetectReason: detectReason,
	}); err != nil {
		return emptyString, fmt.Errorf("failed to save answer to db: %w", err)
	}
	logger.Info("Save answer info to db successfully")

	if authorID == "canglimo" {
		go p.saveEmbedding(answer.ID, formattedText, logger)
	}

	return formattedText, nil
}

func AddPaidColumnContentNotice(text, answerType string) string {
	if answerType != answerTypePaidColumnContent {
		return text
	}

	text = strings.TrimLeft(text, " \n")
	if text == "" {
		return paidColumnContentNotice
	}
	return md.Join(paidColumnContentNotice, text)
}

func (p *ParseService) saveEmbedding(answerID int, text string, logger *zap.Logger) {
	embedding, err := p.ai.Embed(text)
	if err != nil {
		logger.Error("Failed to embed text", zap.Error(err))
		return
	}

	answerIDStr := strconv.Itoa(answerID)
	_, err = p.embeddingDB.CreateEmbedding(common.ZhihuAnswer, answerIDStr, embedding)
	if err != nil {
		logger.Error("Failed to create embedding", zap.Error(err))
	}
	logger.Info("Save embedding to db successfully", zap.String("answer_id", answerIDStr))
}
