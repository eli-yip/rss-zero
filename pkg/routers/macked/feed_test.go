package macked

import (
	"fmt"
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/internal/golden"
	"github.com/eli-yip/rss-zero/internal/rss"
)

func mackedSamplePosts() []ParsedPost {
	return []ParsedPost{
		{ID: "5001", Title: "AppOne 2.3.4", Content: "<p>AppOne cracked</p>", Link: "https://macked.app/appone", Modified: time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)},
		{ID: "4002", Title: "AppTwo 1.0.0", Content: "<p>AppTwo cracked</p>", Link: "https://macked.app/apptwo", Modified: time.Date(2026, 6, 21, 9, 0, 0, 0, time.UTC)},
	}
}

// TestFeedFromPostsGolden locks the macked feed output, including the deterministic
// composite entry id.
func TestFeedFromPostsGolden(t *testing.T) {
	meta, items := feedFromPosts(mackedSamplePosts())
	got, err := rss.RenderAtom(meta, items)
	if err != nil {
		t.Fatalf("RenderAtom: %v", err)
	}
	golden.Assert(t, "macked", got)
}

// TestFeedFromPostsCompositeID pins the deterministic composite id and the two
// properties the random id used to provide: stable across renders, but new when
// the post is modified.
func TestFeedFromPostsCompositeID(t *testing.T) {
	p := ParsedPost{ID: "5001", Title: "x", Content: "<p>x</p>", Link: "https://macked.app/x", Modified: time.Date(2026, 6, 22, 10, 0, 0, 0, time.UTC)}

	_, items := feedFromPosts([]ParsedPost{p})
	want := fmt.Sprintf("%s-%d", p.ID, p.Modified.Unix())
	if items[0].ID != want {
		t.Fatalf("id = %q, want %q", items[0].ID, want)
	}

	_, items2 := feedFromPosts([]ParsedPost{p})
	if items[0].ID != items2[0].ID {
		t.Fatalf("id not deterministic across renders: %q vs %q", items[0].ID, items2[0].ID)
	}

	bumped := p
	bumped.Modified = p.Modified.Add(time.Hour)
	_, items3 := feedFromPosts([]ParsedPost{bumped})
	if items3[0].ID == items[0].ID {
		t.Fatalf("modified post should get a new id, both = %q", items[0].ID)
	}
}
