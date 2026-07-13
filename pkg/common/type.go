package common

import (
	"database/sql/driver"
	"fmt"
	"strconv"
)

// ZhihuContentType identifies a Zhihu content kind. Valid values are the
// constants below, or values produced by ParseZhihuSlug / Scan.
type ZhihuContentType string

const (
	ZhihuAnswer  ZhihuContentType = "answer"
	ZhihuArticle ZhihuContentType = "article"
	ZhihuPin     ZhihuContentType = "pin"
)

const (
	// These values are the existing database encoding; do not reorder.
	legacyZhihuAnswer = iota
	legacyZhihuArticle
	legacyZhihuPin
)

type zhihuSpec struct {
	legacyID    int
	profilePath string
	titleZH     string
}

var zhihuSpecs = map[ZhihuContentType]zhihuSpec{
	ZhihuAnswer:  {legacyID: legacyZhihuAnswer, profilePath: "answers", titleZH: "回答"},
	ZhihuArticle: {legacyID: legacyZhihuArticle, profilePath: "posts", titleZH: "文章"},
	ZhihuPin:     {legacyID: legacyZhihuPin, profilePath: "pins", titleZH: "想法"},
}

var zhihuByLegacyID = func() map[int]ZhihuContentType {
	m := make(map[int]ZhihuContentType, len(zhihuSpecs))
	for t, s := range zhihuSpecs {
		if prev, ok := m[s.legacyID]; ok {
			panic(fmt.Sprintf("duplicate zhihu legacy id %d for %s and %s", s.legacyID, prev, t))
		}
		m[s.legacyID] = t
	}
	return m
}()

func ParseZhihuSlug(s string) (ZhihuContentType, error) {
	t := ZhihuContentType(s)
	if !t.Valid() {
		return "", fmt.Errorf("unknown zhihu content type slug: %q", s)
	}
	return t, nil
}

// ParseZhihuLegacyID converts the legacy database integer to a content type.
// Keep new business code on ZhihuContentType; this exists for migration edges.
func ParseZhihuLegacyID(id int) (ZhihuContentType, error) {
	t, ok := zhihuByLegacyID[id]
	if !ok {
		return "", fmt.Errorf("unknown zhihu content type legacy id: %d", id)
	}
	return t, nil
}

// ZhihuLegacyID converts a content type to the legacy database integer.
// Keep new business code on ZhihuContentType; this exists for migration edges.
func ZhihuLegacyID(t ZhihuContentType) (int, error) {
	spec, ok := zhihuSpecs[t]
	if !ok {
		return 0, fmt.Errorf("unknown zhihu content type: %q", t)
	}
	return spec.legacyID, nil
}

func (t ZhihuContentType) Slug() string {
	t.mustSpec()
	return string(t)
}

// RedisKey 是 zhihu 每作者 RSS feed 的源缓存键，cron warm（crawl.go）与 controller cache-miss
// （rss.go）共用同一个入口，故换命名空间只此一处。v2（plan 决策 6）：删 text 列后正文改从 raw
// 重放，换 key 隔离旧 text 生成的陈旧 canonical items。
func (t ZhihuContentType) RedisKey(authorID string) string {
	return "zhihu_rss_v2_" + t.Slug() + "_" + authorID
}

func (t ZhihuContentType) ProfilePath() string {
	return t.mustSpec().profilePath
}

func (t ZhihuContentType) TitleZH() string {
	return t.mustSpec().titleZH
}

func (t ZhihuContentType) String() string {
	return string(t)
}

func (t ZhihuContentType) Valid() bool {
	_, ok := zhihuSpecs[t]
	return ok
}

func (t ZhihuContentType) FeedKey() string {
	return t.Slug() + "_feed"
}

func (t ZhihuContentType) Value() (driver.Value, error) {
	id, err := ZhihuLegacyID(t)
	if err != nil {
		return nil, err
	}
	return int64(id), nil
}

func (t *ZhihuContentType) Scan(value any) error {
	if t == nil {
		return fmt.Errorf("scan zhihu content type into nil pointer")
	}

	id, err := scanLegacyZhihuID(value)
	if err != nil {
		return err
	}

	contentType, err := ParseZhihuLegacyID(id)
	if err != nil {
		return err
	}
	*t = contentType
	return nil
}

func (t ZhihuContentType) mustSpec() zhihuSpec {
	spec, ok := zhihuSpecs[t]
	if !ok {
		panic(fmt.Sprintf("unknown zhihu content type: %q", t))
	}
	return spec
}

func scanLegacyZhihuID(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int8:
		return int(v), nil
	case int16:
		return int(v), nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case uint:
		return int(v), nil
	case uint8:
		return int(v), nil
	case uint16:
		return int(v), nil
	case uint32:
		return int(v), nil
	case uint64:
		return int(v), nil
	case []byte:
		return strconv.Atoi(string(v))
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("cannot scan zhihu content type from %T", value)
	}
}
