package archive

import (
	"fmt"
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

func buildAuthorMap(answers []zhihuDB.Answer, d zhihuDB.DB) (authorMap map[string]string, err error) {
	// Perf: Cache author names
	authorMap = make(map[string]string)
	for _, answer := range answers {
		if _, ok := authorMap[answer.AuthorID]; !ok {
			authorName, err := d.GetAuthorName(answer.AuthorID)
			if err != nil {
				return nil, fmt.Errorf("failed to get author name: %w", err)
			}
			authorMap[answer.AuthorID] = authorName
		}
	}
	return authorMap, nil
}

func buildTopics(answers []zhihuDB.Answer, d zhihuDB.DB) (topics []Topic, err error) {
	questionMap, err := buildQuestionMap(answers, d)
	if err != nil {
		return nil, fmt.Errorf("failed to build question map: %w", err)
	}

	authorMap, err := buildAuthorMap(answers, d)
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
