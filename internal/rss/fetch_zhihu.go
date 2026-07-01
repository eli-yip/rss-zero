package rss

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/render"
	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	zhihuRender "github.com/eli-yip/rss-zero/pkg/routers/zhihu/render"
)

var errUnknownZhihuType = errors.New("unknown zhihu type")

// ZhihuRow is the type-agnostic shape BuildZhihuFeed consumes (answer/article/pin
// flattened to id + official link + title + text + time). Exported so the random
// endpoint can build a feed through the same path.
type ZhihuRow struct {
	ID           int
	OfficialLink string
	CreateTime   time.Time
	Title        string
	Text         string
}

// FetchZhihu builds the canonical feed for a zhihu author's answers/articles/pins,
// loading up to MaxFetch items. Content decoration (origin link appended, archive
// proxy as the entry link, excerpt summary) is preserved; the former calculateTime
// hack is dropped — it has been the identity since 2024-06-22.
func FetchZhihu(contentType common.ZhihuContentType, authorID string, db zhihuDB.DB, logger *zap.Logger) (FeedMeta, []Item, error) {
	authorName, err := db.GetAuthorName(authorID)
	if err != nil {
		return FeedMeta{}, nil, fmt.Errorf("failed to get zhihu author name from database: %w", err)
	}

	rows, err := zhihuRows(contentType, authorID, db)
	if err != nil {
		return FeedMeta{}, nil, err
	}
	if len(rows) == 0 {
		logger.Info("found no zhihu content, building empty feed")
	}

	return BuildZhihuFeed(contentType, authorID, authorName, rows)
}

func zhihuRows(contentType common.ZhihuContentType, authorID string, db zhihuDB.DB) ([]ZhihuRow, error) {
	switch contentType {
	case common.ZhihuAnswer:
		answers, err := db.GetLatestNVisibleAnswer(MaxFetch, authorID)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest answers from database: %w", err)
		}
		// Batch-load the answers' questions in one query instead of one GetQuestion
		// per answer (up to MaxFetch round-trips per feed build).
		questionIDs := lo.Map(answers, func(a zhihuDB.Answer, _ int) int { return a.QuestionID })
		questions, err := db.GetQuestions(questionIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to get questions from database: %w", err)
		}
		titleByQuestionID := lo.SliceToMap(questions, func(q zhihuDB.Question) (int, string) {
			return q.ID, q.Title
		})
		rows := make([]ZhihuRow, 0, len(answers))
		for _, a := range answers {
			title, ok := titleByQuestionID[a.QuestionID]
			if !ok {
				// A stored answer with no question row breaks the answer→question
				// invariant enforced at crawl time (question is saved before the
				// answer). Fail loudly rather than render a blank-title entry.
				return nil, fmt.Errorf("question %d not found for answer %d", a.QuestionID, a.ID)
			}
			rows = append(rows, ZhihuRow{
				ID:           a.ID,
				OfficialLink: zhihuRender.GenerateAnswerLink(a.QuestionID, a.ID),
				CreateTime:   a.CreateAt,
				Title:        title,
				Text:         a.Text,
			})
		}
		return rows, nil
	case common.ZhihuArticle:
		articles, err := db.GetLatestNArticle(MaxFetch, authorID)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest articles from database: %w", err)
		}
		rows := make([]ZhihuRow, 0, len(articles))
		for _, ar := range articles {
			rows = append(rows, ZhihuRow{
				ID:           ar.ID,
				OfficialLink: zhihuRender.GenerateArticleLink(ar.ID),
				CreateTime:   ar.CreateAt,
				Title:        ar.Title,
				Text:         ar.Text,
			})
		}
		return rows, nil
	case common.ZhihuPin:
		pins, err := db.GetLatestNPin(MaxFetch, authorID)
		if err != nil {
			return nil, fmt.Errorf("failed to get latest pins from database: %w", err)
		}
		rows := make([]ZhihuRow, 0, len(pins))
		for _, pin := range pins {
			title := pin.Title
			if title == "" {
				title = strconv.Itoa(pin.ID)
			}
			rows = append(rows, ZhihuRow{
				ID:           pin.ID,
				OfficialLink: zhihuRender.GeneratePinLink(pin.ID),
				CreateTime:   pin.CreateAt,
				Title:        title,
				Text:         pin.Text,
			})
		}
		return rows, nil
	default:
		return nil, errUnknownZhihuType
	}
}

// BuildZhihuFeed builds the envelope and items from already-resolved rows. Shared
// by FetchZhihu and the random endpoint (which supplies its own random ids/time).
func BuildZhihuFeed(contentType common.ZhihuContentType, authorID, authorName string, rows []ZhihuRow) (FeedMeta, []Item, error) {
	feedTitle := "[知乎-" + contentType.TitleZH() + "]" + authorName
	profileLink := fmt.Sprintf("https://www.zhihu.com/people/%s/%s", authorID, contentType.ProfilePath())
	if len(rows) == 0 {
		return FeedMeta{Title: feedTitle, Link: profileLink, Updated: defaultTime}, nil, nil
	}

	meta := FeedMeta{Title: feedTitle, Link: profileLink, Updated: rows[0].CreateTime}
	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		contentHTML, err := render.FeedHTML(render.AppendOriginLink(row.Text, row.OfficialLink))
		if err != nil {
			return FeedMeta{}, nil, fmt.Errorf("failed to render zhihu content: %w", err)
		}
		items = append(items, Item{
			ID:          fmt.Sprintf("%d", row.ID),
			Link:        render.BuildArchiveLink(config.C.Settings.ServerURL, row.OfficialLink),
			Title:       row.Title,
			Author:      authorName,
			Time:        row.CreateTime,
			Summary:     render.ExtractExcerpt(row.Text),
			ContentHTML: contentHTML,
		})
	}
	return meta, items, nil
}
