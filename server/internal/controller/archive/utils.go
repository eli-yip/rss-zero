package archive

import (
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/render"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	zhihuRender "github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
	"github.com/samber/lo"
)

func buildQuestionMap(answers []zhihuDB.Answer, d zhihuDB.DB) (questionMap map[int]zhihuDB.Question, err error) {
	// Perf: Batch fetch questions
	questionIDs := lo.UniqMap(answers, func(answer zhihuDB.Answer, _ int) int {
		return answer.QuestionID
	})
	questions, err := d.GetQuestions(questionIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get questions: %w", err)
	}
	questionMap = lo.Associate(questions, func(question zhihuDB.Question) (int, zhihuDB.Question) {
		return question.ID, question
	})
	return questionMap, nil
}

func buildAuthorMap(authors []string, d zhihuDB.DB) (authorMap map[string]string, err error) {
	// Perf: Cache author names
	authorMap = make(map[string]string)
	for a := range slices.Values(authors) {
		if _, ok := authorMap[a]; !ok {
			authorName, err := d.GetAuthorName(a)
			if err != nil {
				return nil, fmt.Errorf("failed to get author name: %w", err)
			}
			authorMap[a] = authorName
		}
	}
	return authorMap, nil
}

func buildTopicsFromAnswer(answers []zhihuDB.Answer, d zhihuDB.DB) (topics []Topic, err error) {
	questionMap, err := buildQuestionMap(answers, d)
	if err != nil {
		return nil, fmt.Errorf("failed to build question map: %w", err)
	}

	authorIDs := lo.UniqMap(answers, func(answer zhihuDB.Answer, _ int) string { return answer.AuthorID })
	authorMap, err := buildAuthorMap(authorIDs, d)
	if err != nil {
		return nil, fmt.Errorf("failed to build author map: %w", err)
	}

	for _, answer := range answers {
		question, ok := questionMap[answer.QuestionID]
		if !ok {
			return nil, fmt.Errorf("question not found in question map: %d", answer.QuestionID)
		}

		topics = append(topics, Topic{
			ID:          strconv.Itoa(answer.ID),
			OriginalURL: zhihuRender.GenerateAnswerLink(question.ID, answer.ID),
			ArchiveURL:  render.BuildArchiveLink(config.C.Settings.ServerURL, zhihuRender.GenerateAnswerLink(question.ID, answer.ID)),
			Platform:    PlatformZhihu,
			Title:       question.Title,
			CreatedAt:   answer.CreateAt.Format(time.RFC3339),
			Body:        answer.Text,
			Author:      Author{ID: answer.AuthorID, Nickname: authorMap[answer.AuthorID]},
		})
	}

	return topics, nil
}

func buildTopicsFromPin(pins []zhihuDB.Pin, d zhihuDB.DB) (topics []Topic, err error) {
	authorIDs := lo.UniqMap(pins, func(pin zhihuDB.Pin, _ int) string { return pin.AuthorID })
	authorMap, err := buildAuthorMap(authorIDs, d)
	if err != nil {
		return nil, fmt.Errorf("failed to build author map: %w", err)
	}

	for p := range slices.Values(pins) {
		topics = append(topics, Topic{
			ID:          strconv.Itoa(p.ID),
			OriginalURL: zhihuRender.GeneratePinLink(p.ID),
			ArchiveURL:  render.BuildArchiveLink(config.C.Settings.ServerURL, zhihuRender.GeneratePinLink(p.ID)),
			Platform:    PlatformZhihu,
			Title:       p.Title,
			CreatedAt:   p.CreateAt.Format(time.RFC3339),
			Body:        p.Text,
			Author:      Author{ID: p.AuthorID, Nickname: authorMap[p.AuthorID]},
		})
	}

	return topics, nil
}
