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
	Text       string // rendered markdown body (from raw), not the frozen text column
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

	roots := make([]zsxqDB.Topic, 0, len(topics))
	for _, topic := range topics {
		if !zsxqRender.Support(topic.Type) {
			logger.Info("skip unsupported zsxq topic type", zap.String("type", topic.Type), zap.Int("topic_id", topic.ID))
			continue
		}
		roots = append(roots, topic)
	}

	rows, err := zsxqRows(roots, db)
	if err != nil {
		return FeedMeta{}, nil, err
	}

	return BuildZSXQFeed(groupID, groupName, rows)
}

// zsxqRows batch-loads the side-table facts for roots once and renders each root's
// body from raw (not the frozen text column), returning feed rows ready for
// BuildZSXQFeed. The author name comes from the loaded snapshot (read-time
// zsxq_author), so a missing author degrades to an empty name instead of failing
// the whole feed. Callers must pre-filter roots to render-supported types.
func zsxqRows(roots []zsxqDB.Topic, reader zsxqRender.ContentReader) ([]ZSXQRow, error) {
	snapshot, err := zsxqRender.NewContentLoader(reader).Load(roots)
	if err != nil {
		return nil, fmt.Errorf("failed to load zsxq content snapshot: %w", err)
	}

	rows := make([]ZSXQRow, 0, len(roots))
	for _, root := range roots {
		body, err := zsxqRender.RenderMarkdown(root.ID, snapshot)
		if err != nil {
			return nil, fmt.Errorf("failed to render zsxq topic %d: %w", root.ID, err)
		}
		rows = append(rows, ZSXQRow{
			TopicID:    root.ID,
			GroupID:    root.GroupID,
			Title:      root.Title,
			AuthorName: snapshot.Authors[root.AuthorID].Name,
			Time:       root.Time,
			Text:       body,
		})
	}
	return rows, nil
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
