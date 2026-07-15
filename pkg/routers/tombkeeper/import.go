package tombkeeper

import (
	"fmt"
	"strconv"

	"github.com/lib/pq"
	"github.com/samber/lo"
	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/file"
)

const (
	maxFailureExamples     = 3
	maxFailureExampleRunes = 500
	maxNotificationRunes   = 1000
)

// FailureSummary 汇总一次抓取中可恢复的单条失败，并限制通知中的示例数量。
type FailureSummary struct {
	Count    int
	Examples []string
}

func (s *FailureSummary) Add(message string) { s.AddN(1, message) }

func (s *FailureSummary) AddN(count int, message string) {
	if count <= 0 {
		return
	}
	s.Count += count
	if len(s.Examples) < maxFailureExamples {
		s.Examples = append(s.Examples, truncateRunes(message, maxFailureExampleRunes))
	}
}

func (s *FailureSummary) Merge(other FailureSummary) {
	s.Count += other.Count
	remaining := min(maxFailureExamples-len(s.Examples), len(other.Examples))
	if remaining > 0 {
		s.Examples = append(s.Examples, other.Examples[:remaining]...)
	}
}

func truncateRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit-1]) + "…"
}

// ImportStats 是一页时间线的导入统计。EntriesSaved 是本次读库快照观察到的新增成员；
// live/history 并发处理同一篇时可能重复观察，不作为数据库全局计数。
type ImportStats struct {
	EntriesSeen   int
	EntriesSaved  int
	EntriesFailed int
	Failures      FailureSummary
}

type importRequester interface {
	imageRequester
	GetDetail(id string) ([]byte, error)
	GetReppic(longURL string) ([]string, error)
}

// TimelineImporter 将列表页摄取为结构化博文与图片资产，不生成 Markdown。
type TimelineImporter struct {
	req    importRequester
	file   file.File
	store  ImportStore
	logger *zap.Logger
}

func NewTimelineImporter(req Requester, fileService file.File, store ImportStore,
	logger *zap.Logger,
) *TimelineImporter {
	return &TimelineImporter{req: req, file: fileService, store: store, logger: logger}
}

func (i *TimelineImporter) Import(pageHTML []byte) (ImportStats, error) {
	page, err := ExtractTimelinePage(pageHTML)
	if err != nil {
		return ImportStats{}, err
	}
	stats := ImportStats{EntriesSeen: len(page.Entries), EntriesFailed: page.MissingEntries}
	if page.MissingEntries > 0 {
		stats.Failures.AddN(page.MissingEntries,
			fmt.Sprintf("timeline payload missing entries: %d", page.MissingEntries))
	}
	postsProvidedByPage := make(map[int64]importCandidate, len(page.Entries)+len(page.EmbeddedPosts))
	for _, source := range page.EmbeddedPosts {
		postsProvidedByPage[source.ID] = importCandidate{source: source}
	}
	for _, source := range page.Entries {
		postsProvidedByPage[source.ID] = importCandidate{source: source, inTimeline: true}
	}

	referenceIDs := lo.Uniq(lo.FlatMap(page.Entries, func(source SourcePost, _ int) []int64 {
		return referencePostIDs(source.RetweetPostID, source.Links)
	}))
	pagePostIDs := lo.Keys(postsProvidedByPage)
	postIDsToLoad := lo.Uniq(append(pagePostIDs, referenceIDs...))
	storedPosts, err := i.store.GetPosts(postIDsToLoad)
	if err != nil {
		return stats, fmt.Errorf("load existing posts: %w", err)
	}
	storedPostsByID := lo.SliceToMap(storedPosts, func(post Post) (int64, Post) { return post.ID, post })
	referencesToFetch := lo.Filter(referenceIDs, func(id int64, _ int) bool {
		providedByPage := lo.HasKey(postsProvidedByPage, id)
		alreadyStored := lo.HasKey(storedPostsByID, id)
		return !providedByPage && !alreadyStored
	})

	postsToImport := lo.Values(postsProvidedByPage)
	for _, id := range referencesToFetch {
		source, err := i.fetchSourcePost(id)
		if err != nil {
			i.logger.Warn("failed to fetch referenced tombkeeper post", zap.Int64("id", id), zap.Error(err))
			stats.Failures.Add(fmt.Sprintf("fetch referenced post %d: %v", id, err))
			continue
		}
		postsToImport = append(postsToImport, importCandidate{source: source})
	}

	for _, candidate := range postsToImport {
		existingPost, existed := storedPostsByID[candidate.source.ID]
		post := postFromSource(candidate.source, candidate.inTimeline)
		if existed {
			post.H5ImageIDsByURL = cloneH5ImageIDs(existingPost.H5ImageIDsByURL)
		}
		i.resolveMissingH5ImageIDs(&post, &stats.Failures)
		if err := i.store.UpsertPost(&post); err != nil {
			i.logger.Error("failed to save tombkeeper post", zap.Int64("id", post.ID), zap.Error(err))
			stats.Failures.Add(fmt.Sprintf("upsert post %d: %v", post.ID, err))
			if candidate.inTimeline {
				stats.EntriesFailed++
			}
			continue
		}
		if candidate.inTimeline && (!existed || !existingPost.InTimeline) {
			stats.EntriesSaved++
		}
		i.archiveReferencedImages(post, &stats.Failures)
	}
	return stats, nil
}

type importCandidate struct {
	source     SourcePost
	inTimeline bool
}

func postFromSource(source SourcePost, inTimeline bool) Post {
	return Post{
		ID: source.ID, Bid: source.Bid, AuthorID: source.AuthorID,
		ScreenName: source.ScreenName, PublishedAt: source.PublishedAt,
		Text: source.Text, Pics: pq.StringArray(append([]string(nil), source.Pics...)),
		Links: append([]PostLink(nil), source.Links...), H5ImageIDsByURL: make(map[string][]string),
		RetweetPostID: source.RetweetPostID, InTimeline: inTimeline,
	}
}

func cloneH5ImageIDs(idsByURL map[string][]string) map[string][]string {
	return lo.MapValues(idsByURL, func(ids []string, _ string) []string {
		return cloneNonNilImageIDs(ids)
	})
}

func cloneNonNilImageIDs(ids []string) []string {
	return append(make([]string, 0, len(ids)), ids...)
}

func (i *TimelineImporter) resolveMissingH5ImageIDs(post *Post, failures *FailureSummary) {
	for _, link := range post.Links {
		if !isViewPic(link) {
			continue
		}
		if _, resolved := post.H5ImageIDsByURL[link.LongURL]; resolved {
			continue
		}
		ids, err := i.req.GetReppic(link.LongURL)
		if err != nil {
			i.logger.Warn("failed to resolve 查看图片 H5 page", zap.String("long_url", link.LongURL), zap.Error(err))
			failures.Add(fmt.Sprintf("resolve H5 image index for post %d (%s): %v", post.ID, link.LongURL, err))
			continue
		}
		post.H5ImageIDsByURL[link.LongURL] = cloneNonNilImageIDs(ids)
	}
}

func (i *TimelineImporter) archiveReferencedImages(post Post, failures *FailureSummary) {
	images := append([]string(nil), post.Pics...)
	for _, ids := range post.H5ImageIDsByURL {
		images = append(images, ids...)
	}
	for _, image := range lo.Uniq(images) {
		if err := archiveImageAsset(i.req, i.file, i.store, image, i.logger); err != nil {
			i.logger.Error("failed to archive tombkeeper image asset",
				zap.Int64("post_id", post.ID), zap.String("image", image), zap.Error(err))
			failures.Add(fmt.Sprintf("archive image %s for post %d: %v", image, post.ID, err))
		}
	}
}

func (i *TimelineImporter) fetchSourcePost(id int64) (SourcePost, error) {
	html, err := i.req.GetDetail(strconv.FormatInt(id, 10))
	if err != nil {
		return SourcePost{}, err
	}
	posts, err := extractSourcePosts(html)
	if err != nil {
		return SourcePost{}, err
	}
	for _, source := range posts {
		if source.ID == id {
			return source, nil
		}
	}
	return SourcePost{}, fmt.Errorf("detail page missing post %d", id)
}

func referencePostIDs(retweetPostID int64, links []PostLink) []int64 {
	ids := make([]int64, 0, len(links)+1)
	if retweetPostID != 0 {
		ids = append(ids, retweetPostID)
	}
	for _, link := range links {
		if !isWeiboTextLink(link) {
			continue
		}
		if id, ok := weiboLinkPostID(link.LongURL); ok {
			ids = append(ids, id)
		}
	}
	return lo.Uniq(ids)
}
