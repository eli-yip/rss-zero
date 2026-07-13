package render

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	zsxqDB "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
)

// ContentReader 是 ContentLoader 装配快照所需的批量只读端口。
// zsxqDB.DB 已实现全部方法，读取期直接把 db 服务当 reader 注入；
// 单测则注入 fake，故这里收窄成 interface 而非 *gorm.DB。
type ContentReader interface {
	GetObjectsByIDs(ids []int) ([]zsxqDB.Object, error)
	GetArticlesByIDs(ids []string) ([]zsxqDB.Article, error)
	GetAuthorsByIDs(ids []int) ([]zsxqDB.Author, error)
}

// ContentLoader 把一批 topic 根行两阶段批量装配成自包含 ContentSnapshot，
// 供纯函数 RenderMarkdown 消费，避免 per-topic N+1。
type ContentLoader struct{ reader ContentReader }

func NewContentLoader(reader ContentReader) ContentLoader { return ContentLoader{reader: reader} }

// Load 两阶段装配：
//  1. 登记全部 root 到 Topics，反序列化各 root 的 raw，收集其引用的
//     object/article/author id（去重）；
//  2. 每类引用各一次批量查询读入 Objects/Articles/Authors。
//
// 查询次数是常数（最多 3 次），不随 root 数线性增长。缺失的资源保持缺失，
// 不触发二次查询——由 RenderMarkdown 在用到时报错（作者名缺失则降级为空）。
func (l ContentLoader) Load(roots []zsxqDB.Topic) (ContentSnapshot, error) {
	snap := ContentSnapshot{
		Topics:   make(map[int]zsxqDB.Topic, len(roots)),
		Objects:  map[int]zsxqDB.Object{},
		Articles: map[string]zsxqDB.Article{},
		Authors:  map[int]zsxqDB.Author{},
	}

	objectIDs := map[int]struct{}{}
	articleIDs := map[string]struct{}{}
	authorIDs := map[int]struct{}{}

	for _, root := range roots {
		snap.Topics[root.ID] = root
		authorIDs[root.AuthorID] = struct{}{}

		var mt models.Topic
		if err := json.Unmarshal(root.Raw, &mt); err != nil {
			return ContentSnapshot{}, fmt.Errorf("failed to decode topic %d raw: %w", root.ID, err)
		}
		collectRefs(mt, objectIDs, articleIDs)
	}

	if len(objectIDs) > 0 {
		objects, err := l.reader.GetObjectsByIDs(slices.Collect(maps.Keys(objectIDs)))
		if err != nil {
			return ContentSnapshot{}, fmt.Errorf("failed to batch load objects: %w", err)
		}
		for _, o := range objects {
			snap.Objects[o.ID] = o
		}
	}
	if len(articleIDs) > 0 {
		articles, err := l.reader.GetArticlesByIDs(slices.Collect(maps.Keys(articleIDs)))
		if err != nil {
			return ContentSnapshot{}, fmt.Errorf("failed to batch load articles: %w", err)
		}
		for _, a := range articles {
			snap.Articles[a.ID] = a
		}
	}
	if len(authorIDs) > 0 {
		authors, err := l.reader.GetAuthorsByIDs(slices.Collect(maps.Keys(authorIDs)))
		if err != nil {
			return ContentSnapshot{}, fmt.Errorf("failed to batch load authors: %w", err)
		}
		for _, a := range authors {
			snap.Authors[a.ID] = a
		}
	}

	return snap, nil
}

// collectRefs 把一条 topic 引用的资源/外部文章 id 收进去重集合；
// 与 RenderMarkdown 消费的字段一一对应（talk 的文件/图片/文章、提问图片、回答语音/图片）。
func collectRefs(mt models.Topic, objectIDs map[int]struct{}, articleIDs map[string]struct{}) {
	if mt.Talk != nil {
		for _, f := range mt.Talk.Files {
			objectIDs[f.FileID] = struct{}{}
		}
		for _, img := range mt.Talk.Images {
			objectIDs[img.ImageID] = struct{}{}
		}
		if mt.Talk.Article != nil {
			articleIDs[mt.Talk.Article.ArticleID] = struct{}{}
		}
	}
	if mt.Question != nil {
		for _, img := range mt.Question.Images {
			objectIDs[img.ImageID] = struct{}{}
		}
	}
	if mt.Answer != nil {
		if mt.Answer.Voice != nil {
			objectIDs[mt.Answer.Voice.VoiceID] = struct{}{}
		}
		for _, img := range mt.Answer.Images {
			objectIDs[img.ImageID] = struct{}{}
		}
	}
}
