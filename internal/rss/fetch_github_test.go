package rss

import (
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/internal/golden"
	githubDB "github.com/eli-yip/rss-zero/pkg/routers/github/db"
)

// TestFeedFromGitHubReleasesGolden locks the github feed output. The golden was
// verified byte-for-byte against the former render.Render at migration time.
func TestFeedFromGitHubReleasesGolden(t *testing.T) {
	t1 := time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)
	t2 := time.Date(2026, 6, 10, 8, 0, 0, 0, time.UTC)
	releases := []githubDB.Release{
		{ID: 101, URL: "https://github.com/owner/repo/releases/tag/v1.2.0", Tag: "v1.2.0", Title: "Release 1.2.0", Body: "body **md**", RawBody: "raw1", PreRelease: false, PublishedAt: t1},
		{ID: 102, URL: "https://github.com/owner/repo/releases/tag/v1.1.0", Tag: "v1.1.0", Title: "", Body: "", RawBody: "raw body only", PreRelease: false, PublishedAt: t2},
	}

	meta, items, err := feedFromGitHubReleases("owner", "repo", false, releases)
	if err != nil {
		t.Fatalf("feedFromGitHubReleases: %v", err)
	}
	got, err := RenderAtom(meta, items)
	if err != nil {
		t.Fatalf("RenderAtom: %v", err)
	}
	golden.Assert(t, "github", got)
}
