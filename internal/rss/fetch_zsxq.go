package rss

import (
	"fmt"
	"strconv"
	"time"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/render"
	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	zsxqRender "github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
)

// ZSXQRow is a supported topic flattened with its resolved author name. Exported
// so the random endpoint can build a feed through the same path; FakeID overrides
// the entry id (the random endpoint uses a unique id so each pick stays fresh).
type ZSXQRow struct {
	TopicID    int
	GroupID    int
	Title      *string
	AuthorName string
	Time       time.Time
	Text       string
	FakeID     *string
}

// FetchZSXQ builds the canonical feed for a zsxq group, loading up to MaxFetch
// topics and skipping unsupported topic types. The unsupported-type filter
// (render.Support) is applied here to match the cron warm path — the former
// on-demand GenerateZSXQ skipped it, an inconsistency this fixes.
func FetchZSXQ(groupID int, db zsxqDB.DB, logger *zap.Logger) (FeedMeta, []Item, error) {
	groupName, err := db.GetGroupName(groupID)
	if err != nil {
		return FeedMeta{}, nil, fmt.Errorf("failed to get zsxq group name from database: %w", err)
	}

	topics, err := db.GetLatestNTopics(groupID, MaxFetch)
	if err != nil {
		return FeedMeta{}, nil, fmt.Errorf("failed to get latest topics from database: %w", err)
	}

	// zsxq groups are usually single-author, so memoize names to avoid re-querying
	// the same AuthorID up to MaxFetch times per warm.
	authorNames := make(map[int]string)
	rows := make([]ZSXQRow, 0, len(topics))
	for _, topic := range topics {
		if !zsxqRender.Support(topic.Type) {
			logger.Info("skip unsupported zsxq topic type", zap.String("type", topic.Type), zap.Int("topic_id", topic.ID))
			continue
		}
		authorName, ok := authorNames[topic.AuthorID]
		if !ok {
			if authorName, err = db.GetAuthorName(topic.AuthorID); err != nil {
				return FeedMeta{}, nil, fmt.Errorf("failed to get author %d name from database: %w", topic.AuthorID, err)
			}
			authorNames[topic.AuthorID] = authorName
		}
		rows = append(rows, ZSXQRow{
			TopicID:    topic.ID,
			GroupID:    topic.GroupID,
			Title:      topic.Title,
			AuthorName: authorName,
			Time:       topic.Time,
			Text:       topic.Text,
		})
	}

	return BuildZSXQFeed(groupID, groupName, rows)
}

// BuildZSXQFeed builds the envelope and items from already-resolved rows. Shared by
// FetchZSXQ and the random endpoint.
func BuildZSXQFeed(groupID int, groupName string, rows []ZSXQRow) (FeedMeta, []Item, error) {
	if len(rows) == 0 {
		return FeedMeta{Title: groupName, Link: zsxqRender.BuildGroupLink(groupID), Updated: defaultTime}, nil, nil
	}

	meta := FeedMeta{
		Title:   groupName,
		Link:    zsxqRender.BuildGroupLink(rows[0].GroupID),
		Updated: rows[0].Time,
	}

	items := make([]Item, 0, len(rows))
	for _, row := range rows {
		officialLink := zsxqRender.BuildLink(row.GroupID, row.TopicID)
		contentHTML, err := render.FeedHTML(render.AppendOriginLink(row.Text, officialLink))
		if err != nil {
			return FeedMeta{}, nil, fmt.Errorf("failed to render zsxq content: %w", err)
		}
		title := strconv.Itoa(row.TopicID)
		if row.Title != nil {
			title = *row.Title
		}
		id := strconv.Itoa(row.TopicID)
		if row.FakeID != nil {
			id = *row.FakeID
		}
		items = append(items, Item{
			ID:          id,
			Link:        render.BuildArchiveLink(config.C.Settings.ServerURL, officialLink),
			Title:       title,
			Author:      row.AuthorName,
			Time:        row.Time,
			Summary:     render.ExtractExcerpt(row.Text),
			ContentHTML: contentHTML,
		})
	}
	return meta, items, nil
}
