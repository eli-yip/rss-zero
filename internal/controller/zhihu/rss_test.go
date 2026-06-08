package controller

import (
	"testing"

	"github.com/eli-yip/rss-zero/pkg/common"
)

func TestExtractTypeAuthorFromKey(t *testing.T) {
	generator := &RssGenerator{}

	tests := []struct {
		name       string
		key        string
		wantType   common.ZhihuContentType
		wantAuthor string
	}{
		{name: "answer", key: common.ZhihuAnswer.RedisKey("alice"), wantType: common.ZhihuAnswer, wantAuthor: "alice"},
		{name: "article", key: common.ZhihuArticle.RedisKey("bob"), wantType: common.ZhihuArticle, wantAuthor: "bob"},
		{name: "pin", key: common.ZhihuPin.RedisKey("carol"), wantType: common.ZhihuPin, wantAuthor: "carol"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotAuthor, err := generator.extractTypeAuthorFromKey(tt.key)
			if err != nil {
				t.Fatalf("extractTypeAuthorFromKey(%q) returned error: %v", tt.key, err)
			}
			if gotType != tt.wantType {
				t.Fatalf("type = %q, want %q", gotType, tt.wantType)
			}
			if gotAuthor != tt.wantAuthor {
				t.Fatalf("author = %q, want %q", gotAuthor, tt.wantAuthor)
			}
		})
	}
}

func TestExtractTypeAuthorFromKeyInvalid(t *testing.T) {
	generator := &RssGenerator{}

	tests := []string{
		"zhihu_rss_answer",
		"zhihu_rss_zvideo_alice",
	}

	for _, key := range tests {
		if gotType, gotAuthor, err := generator.extractTypeAuthorFromKey(key); err == nil || gotType != "" || gotAuthor != "" {
			t.Fatalf("extractTypeAuthorFromKey(%q) = %q, %q, %v, want empty values and error", key, gotType, gotAuthor, err)
		}
	}
}
