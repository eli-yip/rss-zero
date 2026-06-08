package common

import (
	"database/sql/driver"
	"testing"
)

func TestZhihuContentTypeValues(t *testing.T) {
	tests := []struct {
		name        string
		contentType ZhihuContentType
		slug        string
		redisKey    string
		profilePath string
		titleZH     string
		feedKey     string
		legacyValue driver.Value
	}{
		{
			name:        "answer",
			contentType: ZhihuAnswer,
			slug:        "answer",
			redisKey:    "zhihu_rss_answer_42",
			profilePath: "answers",
			titleZH:     "回答",
			feedKey:     "answer_feed",
			legacyValue: int64(0),
		},
		{
			name:        "article",
			contentType: ZhihuArticle,
			slug:        "article",
			redisKey:    "zhihu_rss_article_42",
			profilePath: "posts",
			titleZH:     "文章",
			feedKey:     "article_feed",
			legacyValue: int64(1),
		},
		{
			name:        "pin",
			contentType: ZhihuPin,
			slug:        "pin",
			redisKey:    "zhihu_rss_pin_42",
			profilePath: "pins",
			titleZH:     "想法",
			feedKey:     "pin_feed",
			legacyValue: int64(2),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.contentType.Valid() {
				t.Fatalf("%s should be valid", tt.contentType)
			}
			if got := tt.contentType.Slug(); got != tt.slug {
				t.Fatalf("Slug() = %q, want %q", got, tt.slug)
			}
			if got := tt.contentType.String(); got != tt.slug {
				t.Fatalf("String() = %q, want %q", got, tt.slug)
			}
			if got := tt.contentType.RedisKey("42"); got != tt.redisKey {
				t.Fatalf("RedisKey() = %q, want %q", got, tt.redisKey)
			}
			if got := tt.contentType.ProfilePath(); got != tt.profilePath {
				t.Fatalf("ProfilePath() = %q, want %q", got, tt.profilePath)
			}
			if got := tt.contentType.TitleZH(); got != tt.titleZH {
				t.Fatalf("TitleZH() = %q, want %q", got, tt.titleZH)
			}
			if got := tt.contentType.FeedKey(); got != tt.feedKey {
				t.Fatalf("FeedKey() = %q, want %q", got, tt.feedKey)
			}
			if got, err := tt.contentType.Value(); err != nil || got != tt.legacyValue {
				t.Fatalf("Value() = %v, %v, want %v, nil", got, err, tt.legacyValue)
			}
		})
	}
}

func TestParseZhihuSlug(t *testing.T) {
	tests := map[string]ZhihuContentType{
		"answer":  ZhihuAnswer,
		"article": ZhihuArticle,
		"pin":     ZhihuPin,
	}

	for slug, want := range tests {
		got, err := ParseZhihuSlug(slug)
		if err != nil {
			t.Fatalf("ParseZhihuSlug(%q) returned error: %v", slug, err)
		}
		if got != want {
			t.Fatalf("ParseZhihuSlug(%q) = %q, want %q", slug, got, want)
		}
	}

	if got, err := ParseZhihuSlug(""); err == nil || got != "" {
		t.Fatalf("ParseZhihuSlug(\"\") = %q, %v, want empty value and error", got, err)
	}
	if got, err := ParseZhihuSlug("zvideo"); err == nil || got != "" {
		t.Fatalf("ParseZhihuSlug(\"zvideo\") = %q, %v, want empty value and error", got, err)
	}
}

func TestZhihuLegacyID(t *testing.T) {
	tests := []struct {
		contentType ZhihuContentType
		legacyID    int
	}{
		{contentType: ZhihuAnswer, legacyID: 0},
		{contentType: ZhihuArticle, legacyID: 1},
		{contentType: ZhihuPin, legacyID: 2},
	}

	for _, tt := range tests {
		gotType, err := ParseZhihuLegacyID(tt.legacyID)
		if err != nil {
			t.Fatalf("ParseZhihuLegacyID(%d) returned error: %v", tt.legacyID, err)
		}
		if gotType != tt.contentType {
			t.Fatalf("ParseZhihuLegacyID(%d) = %q, want %q", tt.legacyID, gotType, tt.contentType)
		}

		gotID, err := ZhihuLegacyID(tt.contentType)
		if err != nil {
			t.Fatalf("ZhihuLegacyID(%q) returned error: %v", tt.contentType, err)
		}
		if gotID != tt.legacyID {
			t.Fatalf("ZhihuLegacyID(%q) = %d, want %d", tt.contentType, gotID, tt.legacyID)
		}
	}

	if got, err := ParseZhihuLegacyID(100); err == nil || got != "" {
		t.Fatalf("ParseZhihuLegacyID(100) = %q, %v, want empty value and error", got, err)
	}
	if got, err := ZhihuLegacyID(ZhihuContentType("zvideo")); err == nil || got != 0 {
		t.Fatalf("ZhihuLegacyID(zvideo) = %d, %v, want zero and error", got, err)
	}
}

func TestZhihuContentTypeSQLRoundTrip(t *testing.T) {
	tests := []struct {
		value any
		want  ZhihuContentType
	}{
		{value: int64(0), want: ZhihuAnswer},
		{value: int64(1), want: ZhihuArticle},
		{value: int64(2), want: ZhihuPin},
		{value: []byte("0"), want: ZhihuAnswer},
		{value: "1", want: ZhihuArticle},
	}

	for _, tt := range tests {
		var got ZhihuContentType
		if err := got.Scan(tt.value); err != nil {
			t.Fatalf("Scan(%v) returned error: %v", tt.value, err)
		}
		if got != tt.want {
			t.Fatalf("Scan(%v) = %q, want %q", tt.value, got, tt.want)
		}
	}
}

func TestZhihuContentTypeInvalidValues(t *testing.T) {
	var zero ZhihuContentType
	if zero.Valid() {
		t.Fatal("zero value should be invalid")
	}
	if _, err := zero.Value(); err == nil {
		t.Fatal("zero value Value() should return an error")
	}

	var scanned ZhihuContentType
	if err := scanned.Scan(int64(3)); err == nil {
		t.Fatal("Scan(3) should return an error")
	}
	if err := scanned.Scan(nil); err == nil {
		t.Fatal("Scan(nil) should return an error")
	}
}

func TestZhihuContentTypeInvalidValuePanics(t *testing.T) {
	invalid := ZhihuContentType("zvideo")

	assertPanics(t, "Slug", func() { _ = invalid.Slug() })
	assertPanics(t, "RedisKey", func() { _ = invalid.RedisKey("42") })
	assertPanics(t, "ProfilePath", func() { _ = invalid.ProfilePath() })
	assertPanics(t, "TitleZH", func() { _ = invalid.TitleZH() })
	assertPanics(t, "FeedKey", func() { _ = invalid.FeedKey() })
}

func TestZhihuContentTypeInvalidString(t *testing.T) {
	invalid := ZhihuContentType("zvideo")
	if got := invalid.String(); got != "zvideo" {
		t.Fatalf("String() = %q, want %q", got, "zvideo")
	}
}

func assertPanics(t *testing.T, name string, f func()) {
	t.Helper()

	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("%s should panic", name)
		}
	}()

	f()
}
