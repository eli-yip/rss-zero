package parse

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

type AnswerParser interface {
	// answerExcerpt: answer list for answer id etc. why: some may need some info before parse raw message
	ParseAnswerList(content []byte, index int, logger *zap.Logger) (paging apiModels.Paging, answersExcerpt []apiModels.Answer, answers []json.RawMessage, err error)
	ParseAnswer(content []byte, authorID string, logger *zap.Logger) error
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

func (p *ParseService) ParseAnswer(content []byte, authorID string, logger *zap.Logger) (err error) {
	answer := apiModels.Answer{}
	if err = json.Unmarshal(content, &answer); err != nil {
		return fmt.Errorf("failed to unmarshal answer: %w", err)
	}
	logger.Info("Unmarshal answer successfully")

	answer.ID, err = anyToID(answer.RawID)
	if err != nil {
		return fmt.Errorf("failed to convert answer id from any to int: %w", err)
	}
	answer.Question.ID, err = anyToID(answer.Question.RawID)
	if err != nil {
		return fmt.Errorf("failed to convert question id from any to int: %w", err)
	}

	answerInDB, err := loadOrAbsent(p.db.GetAnswer, answer.ID)
	if err != nil {
		return fmt.Errorf("failed to get answer from db: %w", err)
	}
	if answerInDB != nil && storedIsCurrent(answerInDB.UpdateAt, time.Unix(answer.UpdateAt, 0)) {
		logger.Info("Answer already up-to-date, skip re-parsing")
		return nil
	}

	// 抓取期仍下载并转存图片对象（副作用不变），但对象元数据不即时写库、随根行同事务提交；
	// 换链已移到读取期纯渲染。convertedBytes 既用于定位待下载图片，也作为下方 transient 正文喂给
	// RenderMarkdown（注入快照 Bodies 复用），同一段 HTML 抓取期只转换一次。
	convertedBytes, err := p.htmlToMarkdown.Convert([]byte(answer.HTML))
	if err != nil {
		return fmt.Errorf("failed to convert html to markdown: %w", err)
	}
	objects, err := p.downloadImageObjects(string(convertedBytes), answer.ID, common.ZhihuAnswer, logger)
	if err != nil {
		return fmt.Errorf("failed to download images: %w", err)
	}
	logger.Info("Parse html content successfully")

	// transient 正文：用刚抽取的事实（raw + 刚转存的对象 + 已转换正文）在内存装配快照，跑读取期
	// 同一个纯 RenderMarkdown 得临时正文——喂 word_count / detect / embedding，但不持久化（已无
	// text 列）。与读取期正文逐字节一致，派生事实不漂移（plan 决策 3 / D8）。renderAnswer 只读
	// Answers + Objects + Bodies（正文换链）；问题标题不进正文（AnswerTitle 走读取期单独装配），
	// 故 transient 快照无需 Questions。
	snapshot := render.ContentSnapshot{
		Answers: map[int]db.Answer{answer.ID: {ID: answer.ID, QuestionID: answer.Question.ID, Raw: content}},
		Objects: objectsByID(objects),
		Bodies:  map[int]string{answer.ID: string(convertedBytes)},
	}
	body, err := render.RenderMarkdown(answer.ID, snapshot, emptyString)
	if err != nil {
		return fmt.Errorf("failed to render answer markdown: %w", err)
	}
	logger.Info("Render markdown content successfully")

	detectStatus := db.DetectStatusNone
	detectReason := ""
	if p.detector != nil {
		res, detected, derr := p.detector.Detect(authorID, body)
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

	// 原子提交：问题 + 图片对象 + answer 根行同一事务，一起提交或一起回滚（plan 决策 4）；
	// 事务内根行最后写只是可读性约定，无 FK 强制、不改变回滚语义。
	if err = p.db.SaveAnswerTx(&db.Answer{
		ID:           answer.ID,
		QuestionID:   answer.Question.ID,
		AuthorID:     authorID,
		CreateAt:     time.Unix(answer.CreateAt, 0),
		UpdateAt:     time.Unix(answer.UpdateAt, 0),
		Raw:          content,     // NOTE: see db.Answer.Raw comment
		Status:       db.AnswerStatusCompleted,
		WordCount:    md.Count(body),
		DetectStatus: detectStatus,
		DetectReason: detectReason,
	}, &db.Question{
		ID:       answer.Question.ID,
		CreateAt: time.Unix(answer.Question.CreateAt, 0),
		Title:    answer.Question.Title,
	}, objects); err != nil {
		return fmt.Errorf("failed to save answer to db: %w", err)
	}
	logger.Info("Save answer info to db successfully")

	// embedding 仍在事务提交后 best-effort，喂同一临时正文（plan 决策 4）。
	if authorID == "canglimo" {
		go p.saveEmbedding(answer.ID, body, logger)
	}

	return nil
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
