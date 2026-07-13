package render

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	zhihuDB "github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
)

// ContentReader 是 ContentLoader 装配快照所需的批量只读端口。
// zhihuDB.DB 已实现全部方法，读取期直接把 db 服务当 reader 注入；
// 单测则注入 fake，故这里收窄成 interface 而非 *gorm.DB。
type ContentReader interface {
	GetQuestions(ids []int) ([]zhihuDB.Question, error)  // answer 标题外键
	GetObjectsByIDs(ids []int) ([]zhihuDB.Object, error) // 图片换链事实
}

// ContentLoader 把一批同类型知乎根行两阶段批量装配成自包含 ContentSnapshot，
// 供纯函数 RenderMarkdown 消费，避免逐条 N+1。
//
// 知乎三类内容分表（answer/article/pin），根行类型不同，故按类型各一个 Load 入口；
// 三者共用「收集引用 id → 批量读侧表」的第二阶段。
type ContentLoader struct{ reader ContentReader }

func NewContentLoader(reader ContentReader) ContentLoader { return ContentLoader{reader: reader} }

// LoadAnswers 装配一批 answer 的快照：登记根行、收集各 answer 的 question id（标题外键）
// 与正文图片 object id，再各一次批量读入 Questions/Objects。
func (l ContentLoader) LoadAnswers(roots []zhihuDB.Answer) (ContentSnapshot, error) {
	snap := newSnapshot()
	snap.Answers = make(map[int]zhihuDB.Answer, len(roots))

	questionIDs := map[int]struct{}{}
	objectIDs := map[int]struct{}{}
	for _, root := range roots {
		snap.Answers[root.ID] = root
		questionIDs[root.QuestionID] = struct{}{}

		var am apiModels.Answer
		if err := json.Unmarshal(root.Raw, &am); err != nil {
			return ContentSnapshot{}, fmt.Errorf("failed to decode answer %d raw: %w", root.ID, err)
		}
		if err := convertAndCollect(&snap, root.ID, am.HTML, objectIDs); err != nil {
			return ContentSnapshot{}, fmt.Errorf("failed to collect answer %d image ids: %w", root.ID, err)
		}
	}

	if err := l.loadQuestions(&snap, questionIDs); err != nil {
		return ContentSnapshot{}, err
	}
	if err := l.loadObjects(&snap, objectIDs); err != nil {
		return ContentSnapshot{}, err
	}
	return snap, nil
}

// LoadArticles 装配一批 article 的快照：article 无 question 外键，只需正文图片 object id。
func (l ContentLoader) LoadArticles(roots []zhihuDB.Article) (ContentSnapshot, error) {
	snap := newSnapshot()
	snap.Articles = make(map[int]zhihuDB.Article, len(roots))

	objectIDs := map[int]struct{}{}
	for _, root := range roots {
		snap.Articles[root.ID] = root

		var am apiModels.Article
		if err := json.Unmarshal(root.Raw, &am); err != nil {
			return ContentSnapshot{}, fmt.Errorf("failed to decode article %d raw: %w", root.ID, err)
		}
		if err := convertAndCollect(&snap, root.ID, am.HTML, objectIDs); err != nil {
			return ContentSnapshot{}, fmt.Errorf("failed to collect article %d image ids: %w", root.ID, err)
		}
	}

	if err := l.loadObjects(&snap, objectIDs); err != nil {
		return ContentSnapshot{}, err
	}
	return snap, nil
}

// LoadPins 装配一批 pin 的快照：pin 图片来自结构化 image 块（含内嵌 origin_pin），
// 递归收集其 object id，无需 HTML 转换（与 renderPinImage 的换链键一致）。
func (l ContentLoader) LoadPins(roots []zhihuDB.Pin) (ContentSnapshot, error) {
	snap := newSnapshot()
	snap.Pins = make(map[int]zhihuDB.Pin, len(roots))

	objectIDs := map[int]struct{}{}
	for _, root := range roots {
		snap.Pins[root.ID] = root

		var pm apiModels.Pin
		if err := json.Unmarshal(root.Raw, &pm); err != nil {
			return ContentSnapshot{}, fmt.Errorf("failed to decode pin %d raw: %w", root.ID, err)
		}
		if err := collectPinImageIDs(pm, objectIDs); err != nil {
			return ContentSnapshot{}, fmt.Errorf("failed to collect pin %d image ids: %w", root.ID, err)
		}
	}

	if err := l.loadObjects(&snap, objectIDs); err != nil {
		return ContentSnapshot{}, err
	}
	return snap, nil
}

func newSnapshot() ContentSnapshot {
	return ContentSnapshot{
		Questions: map[int]zhihuDB.Question{},
		Objects:   map[int]zhihuDB.Object{},
		Bodies:    map[int]string{},
	}
}

func (l ContentLoader) loadQuestions(snap *ContentSnapshot, ids map[int]struct{}) error {
	if len(ids) == 0 {
		return nil
	}
	questions, err := l.reader.GetQuestions(slices.Collect(maps.Keys(ids)))
	if err != nil {
		return fmt.Errorf("failed to batch load questions: %w", err)
	}
	for _, q := range questions {
		snap.Questions[q.ID] = q
	}
	return nil
}

func (l ContentLoader) loadObjects(snap *ContentSnapshot, ids map[int]struct{}) error {
	if len(ids) == 0 {
		return nil
	}
	objects, err := l.reader.GetObjectsByIDs(slices.Collect(maps.Keys(ids)))
	if err != nil {
		return fmt.Errorf("failed to batch load objects: %w", err)
	}
	for _, o := range objects {
		snap.Objects[o.ID] = o
	}
	return nil
}

// convertAndCollect 把 answer/article 正文 HTML 转 Markdown 存入 snap.Bodies（供渲染期复用、
// 免二次转换），同时从中收集被换链图片的 object id 进去重集合。渲染期 renderAnswer/renderArticle
// 复用同一份转换结果，故 id 收集口径与渲染换链口径出自同一份正文、天然一致。
func convertAndCollect(snap *ContentSnapshot, id int, html string, ids map[int]struct{}) error {
	converted, err := zhihuHTMLConverter.Convert([]byte(html))
	if err != nil {
		return fmt.Errorf("failed to convert html to markdown: %w", err)
	}
	body := string(converted)
	snap.Bodies[id] = body
	for _, link := range FindImageLinks(body) {
		ids[URLToID(link)] = struct{}{}
	}
	return nil
}

// collectPinImageIDs 递归收集 pin（含 origin_pin，代码递归任意深度、zhihu 实际至多一层）里
// image 块引用的 object id，与 renderPinContent/renderPin 消费的图片块一一对应
// （text/link/video/link_card/poll 不引对象）。
func collectPinImageIDs(pin apiModels.Pin, ids map[int]struct{}) error {
	for _, c := range pin.Content {
		var contentType apiModels.PinContentType
		if err := json.Unmarshal(c, &contentType); err != nil {
			return fmt.Errorf("failed to unmarshal pin content type: %w", err)
		}
		if contentType.Type != "image" {
			continue
		}
		var imageContent apiModels.PinImage
		if err := json.Unmarshal(c, &imageContent); err != nil {
			return fmt.Errorf("failed to unmarshal pin image content: %w", err)
		}
		ids[URLToID(imageContent.OriginalURL)] = struct{}{}
	}
	if pin.OriginPin != nil {
		if err := collectPinImageIDs(*pin.OriginPin, ids); err != nil {
			return err
		}
	}
	return nil
}
