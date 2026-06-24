package endoflife

import (
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/internal/golden"
	"github.com/eli-yip/rss-zero/internal/rss"
)

// TestFeedFromVersionsGolden locks the endoflife feed output.
func TestFeedFromVersionsGolden(t *testing.T) {
	list := []versionInfo{
		{version: version{major: 9, minor: 5, Patch: 2}, releaseDate: time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC), lts: true},
		{version: version{major: 9, minor: 6, Patch: 0}, releaseDate: time.Date(2026, 6, 10, 0, 0, 0, 0, time.UTC), lts: false},
	}

	meta, items, err := feedFromVersions("mattermost", list)
	if err != nil {
		t.Fatalf("feedFromVersions: %v", err)
	}
	got, err := rss.RenderAtom(meta, items)
	if err != nil {
		t.Fatalf("RenderAtom: %v", err)
	}
	golden.Assert(t, "endoflife", got)
}
