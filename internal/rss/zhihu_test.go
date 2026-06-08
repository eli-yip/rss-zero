package rss

import (
	"errors"
	"testing"

	"github.com/eli-yip/rss-zero/pkg/common"
)

func TestGenerateZhihuRSSPath(t *testing.T) {
	tests := []struct {
		name        string
		contentType common.ZhihuContentType
		want        string
	}{
		{name: "answer", contentType: common.ZhihuAnswer, want: "zhihu_rss_answer_alice"},
		{name: "article", contentType: common.ZhihuArticle, want: "zhihu_rss_article_alice"},
		{name: "pin", contentType: common.ZhihuPin, want: "zhihu_rss_pin_alice"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generateZhihuRSSPath(tt.contentType, "alice")
			if err != nil {
				t.Fatalf("generateZhihuRSSPath returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("path = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGenerateZhihuRSSPathInvalid(t *testing.T) {
	got, err := generateZhihuRSSPath(common.ZhihuContentType("zvideo"), "alice")
	if !errors.Is(err, errUnknownZhihuType) {
		t.Fatalf("error = %v, want errUnknownZhihuType", err)
	}
	if got != "" {
		t.Fatalf("path = %q, want empty string", got)
	}
}
