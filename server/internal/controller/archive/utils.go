package archive

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/eli-yip/rss-zero/config"
	bookmarkDB "github.com/eli-yip/rss-zero/pkg/bookmark/db"
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

func buildQuestionMapFromAnswer(answers map[int]zhihuDB.Answer, d zhihuDB.DB) (questionMap map[int]zhihuDB.Question, err error) {
	questionIDs := lo.Uniq(lo.MapToSlice(answers, func(_ int, v zhihuDB.Answer) int { return v.QuestionID }))
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

func buildTopicsFromAnswer(answers []zhihuDB.Answer, userID string, d zhihuDB.DB, bd bookmarkDB.DB) (topics []Topic, err error) {
	questionMap, err := buildQuestionMap(answers, d)
	if err != nil {
		return nil, fmt.Errorf("failed to build question map: %w", err)
	}

	authorIDs := lo.UniqMap(answers, func(answer zhihuDB.Answer, _ int) string { return answer.AuthorID })
	authorMap, err := buildAuthorMap(authorIDs, d)
	if err != nil {
		return nil, fmt.Errorf("failed to build author map: %w", err)
	}

	for answer := range slices.Values(answers) {
		question, ok := questionMap[answer.QuestionID]
		if !ok {
			return nil, fmt.Errorf("question not found in question map: %d", answer.QuestionID)
		}

		answerID := strconv.Itoa(answer.ID)
		bookmark, err := bd.GetBookmarkByContent(userID, bookmarkDB.ContentTypeAnswer, answerID)
		var custom *Custom
		if err != nil {
			if !errors.Is(err, bookmarkDB.ErrNoBookmark) {
				return nil, fmt.Errorf("failed to check bookmark: %w", err)
			}
		} else {
			tags, err := bd.GetTag(bookmark.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to get tags: %w", err)
			}
			custom = &Custom{
				Bookmark:   true,
				BookmarkID: bookmark.ID,
				Tags:       tags,
				Comment:    bookmark.Comment,
				Note:       bookmark.Note,
			}
			if tags == nil {
				custom.Tags = []string{}
			}
		}

		topics = append(topics, Topic{
			ID:          answerID,
			OriginalURL: zhihuRender.GenerateAnswerLink(question.ID, answer.ID),
			ArchiveURL:  render.BuildArchiveLink(config.C.Settings.ServerURL, zhihuRender.GenerateAnswerLink(question.ID, answer.ID)),
			Platform:    PlatformZhihu,
			Type:        bookmarkDB.ContentTypeAnswer,
			Title:       question.Title,
			CreatedAt:   answer.CreateAt.Format(time.RFC3339),
			Body:        answer.Text,
			Author:      Author{ID: answer.AuthorID, Nickname: authorMap[answer.AuthorID]},
			Custom:      custom,
		})
	}

	return topics, nil
}

// answers: map[answerID]Answer
// bookmarks: map[answerID]bookmark
// tags: map[bookmarkID]tags
// Returns a map of answerID to Topic
func buildTopicMapFromAnswer(answers map[int]zhihuDB.Answer, bookmarks map[int]bookmarkDB.Bookmark, tags map[string][]string, d zhihuDB.DB) (topicMap map[string]Topic, err error) {
	questionMap, err := buildQuestionMapFromAnswer(answers, d)
	if err != nil {
		return nil, fmt.Errorf("failed to build question map: %w", err)
	}

	authorIDs := lo.Uniq(lo.MapToSlice(answers, func(_ int, v zhihuDB.Answer) string { return v.AuthorID }))
	authorMap, err := buildAuthorMap(authorIDs, d)
	if err != nil {
		return nil, fmt.Errorf("failed to build author map: %w", err)
	}

	topicMap = make(map[string]Topic) // key: answerID, value: Topic

	for _, answer := range answers {
		question, ok := questionMap[answer.QuestionID]
		if !ok {
			return nil, fmt.Errorf("question not found in question map: %d", answer.QuestionID)
		}

		answerID := strconv.Itoa(answer.ID)

		bookmark := bookmarks[answer.ID]
		custom := &Custom{
			Bookmark:   true,
			BookmarkID: bookmark.ID,
			Comment:    bookmark.Comment,
			Note:       bookmark.Note,
		}
		answerTags, hasTag := tags[bookmark.ID]
		if hasTag {
			custom.Tags = answerTags
		} else {
			custom.Tags = []string{}
		}

		topicMap[answerID] = Topic{
			ID:          answerID,
			OriginalURL: zhihuRender.GenerateAnswerLink(question.ID, answer.ID),
			ArchiveURL:  render.BuildArchiveLink(config.C.Settings.ServerURL, zhihuRender.GenerateAnswerLink(question.ID, answer.ID)),
			Platform:    PlatformZhihu,
			Type:        bookmarkDB.ContentTypeAnswer,
			Title:       question.Title,
			CreatedAt:   answer.CreateAt.Format(time.RFC3339),
			Body:        answer.Text,
			Author:      Author{ID: answer.AuthorID, Nickname: authorMap[answer.AuthorID]},
			Custom:      custom,
		}
	}

	return topicMap, nil
}

func buildTopicsFromPin(pins []zhihuDB.Pin, userID string, d zhihuDB.DB, bd bookmarkDB.DB) (topics []Topic, err error) {
	authorIDs := lo.UniqMap(pins, func(pin zhihuDB.Pin, _ int) string { return pin.AuthorID })
	authorMap, err := buildAuthorMap(authorIDs, d)
	if err != nil {
		return nil, fmt.Errorf("failed to build author map: %w", err)
	}

	for p := range slices.Values(pins) {
		pinID := strconv.Itoa(p.ID)
		bookmark, err := bd.GetBookmarkByContent(userID, bookmarkDB.ContentTypePin, pinID)
		var custom *Custom
		if err != nil {
			if !errors.Is(err, bookmarkDB.ErrNoBookmark) {
				return nil, fmt.Errorf("failed to check bookmark: %w", err)
			}
		} else {
			tags, err := bd.GetTag(bookmark.ID)
			if err != nil {
				return nil, fmt.Errorf("failed to get tags: %w", err)
			}
			custom = &Custom{
				Bookmark:   true,
				BookmarkID: bookmark.ID,
				Tags:       tags,
				Comment:    bookmark.Comment,
				Note:       bookmark.Note,
			}
			if tags == nil {
				custom.Tags = []string{}
			}
		}

		topics = append(topics, Topic{
			ID:          pinID,
			OriginalURL: zhihuRender.GeneratePinLink(p.ID),
			ArchiveURL:  render.BuildArchiveLink(config.C.Settings.ServerURL, zhihuRender.GeneratePinLink(p.ID)),
			Platform:    PlatformZhihu,
			Type:        bookmarkDB.ContentTypePin,
			Title: func() string {
				if p.Title == "" {
					return strconv.Itoa(p.ID)
				}
				return p.Title
			}(),
			CreatedAt: p.CreateAt.Format(time.RFC3339),
			Body:      p.Text,
			Author:    Author{ID: p.AuthorID, Nickname: authorMap[p.AuthorID]},
			Custom:    custom,
		})
	}

	return topics, nil
}

// bookmarks: map[pinID]bookmark
// tags: map[pinID][]string
func buildTopicMapFromPin(pins map[int]zhihuDB.Pin, bookmarks map[int]bookmarkDB.Bookmark, tags map[string][]string, d zhihuDB.DB) (topicMap map[string]Topic, err error) {
	authorIDs := lo.Uniq(lo.MapToSlice(pins, func(_ int, v zhihuDB.Pin) string { return v.AuthorID }))
	authorMap, err := buildAuthorMap(authorIDs, d)
	if err != nil {
		return nil, fmt.Errorf("failed to build author map: %w", err)
	}

	topicMap = make(map[string]Topic)

	for _, p := range pins {
		pinID := strconv.Itoa(p.ID)

		bookmark := bookmarks[p.ID]
		custom := &Custom{
			Bookmark:   true,
			BookmarkID: bookmark.ID,
			Comment:    bookmark.Comment,
			Note:       bookmark.Note,
		}
		pinTags, hasTag := tags[bookmark.ID]
		if hasTag {
			custom.Tags = pinTags
		} else {
			custom.Tags = []string{}
		}

		topicMap[pinID] = Topic{
			ID:          pinID,
			OriginalURL: zhihuRender.GeneratePinLink(p.ID),
			ArchiveURL:  render.BuildArchiveLink(config.C.Settings.ServerURL, zhihuRender.GeneratePinLink(p.ID)),
			Platform:    PlatformZhihu,
			Type:        bookmarkDB.ContentTypePin,
			Title: func() string {
				if p.Title == "" {
					return strconv.Itoa(p.ID)
				}
				return p.Title
			}(),
			CreatedAt: p.CreateAt.Format(time.RFC3339),
			Body:      p.Text,
			Author:    Author{ID: p.AuthorID, Nickname: authorMap[p.AuthorID]},
			Custom:    custom,
		}
	}

	return topicMap, nil
}
