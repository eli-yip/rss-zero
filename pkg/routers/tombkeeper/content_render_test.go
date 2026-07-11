package tombkeeper

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderMarkdownRendersSnapshotWithoutIO(t *testing.T) {
	root := Post{
		ID: 2, AuthorID: "1401527553", Bid: "ROOT", ScreenName: "tombkeeper",
		PublishedAt: time.Date(2026, 6, 26, 10, 0, 0, 0, time.UTC),
		Text:        "root", Pics: []string{"rootpic"}, RetweetPostID: 1,
	}
	original := Post{
		ID: 1, AuthorID: "1401527553", Bid: "ORIGINAL", ScreenName: "tombkeeper",
		PublishedAt: time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC),
		Text:        "original", Pics: []string{"originalpic"},
	}
	content := ContentSnapshot{
		Posts: map[int64]Post{root.ID: root, original.ID: original},
		Images: map[string]ImageAsset{
			"rootpic":     {ID: "rootpic", ObjectKey: "tombkeeper/rootpic.jpg", StorageProvider: []string{"https://oss.test/rss"}, Status: ObjectStatusOK},
			"originalpic": {ID: "originalpic", ObjectKey: "tombkeeper/originalpic.jpg", StorageProvider: []string{"https://oss.test/rss"}, Status: ObjectStatusOK},
		},
	}

	got, err := RenderMarkdown(root.ID, content, "https://srv.test")
	require.NoError(t, err)
	for _, want := range []string{
		"root",
		"![微博图片 1](https://oss.test/rss/tombkeeper/rootpic.jpg)",
		"> 转发 @tombkeeper",
		"> original",
		"> ![微博图片 1](https://oss.test/rss/tombkeeper/originalpic.jpg)",
		"> 2026 年 05 月 28 日 18:00",
	} {
		assert.Contains(t, got, want)
	}
	again, err := RenderMarkdown(root.ID, content, "https://srv.test")
	require.NoError(t, err)
	assert.Equal(t, got, again)
}

func TestContentLoaderIncludesDirectReferenceImages(t *testing.T) {
	root := Post{ID: 2, RetweetPostID: 1}
	original := Post{
		ID: 1, Pics: []string{"originalpic"},
		H5ImageIDsByURL: map[string][]string{"https://photo.weibo.com/h5": {"h5pic"}},
	}
	db := newFakeDB()
	db.posts[original.ID] = &original
	db.objs["originalpic"] = &ImageAsset{ID: "originalpic", Status: ObjectStatusOK}
	db.objs["h5pic"] = &ImageAsset{ID: "h5pic", Status: ObjectStatusOK}

	content, err := NewContentLoader(db).Load([]Post{root})
	require.NoError(t, err)
	assert.Contains(t, content.Posts, root.ID)
	assert.Contains(t, content.Posts, original.ID)
	for _, id := range []string{"originalpic", "h5pic"} {
		assert.Contains(t, content.Images, id)
	}
}

func TestContentLoaderDoesNotReloadOneRootThroughAnotherRoot(t *testing.T) {
	db := newFakeDB()
	db.posts[2] = &Post{ID: 2, Text: "stale database value"}
	roots := []Post{{ID: 1, RetweetPostID: 2}, {ID: 2, Text: "caller root value"}}

	content, err := NewContentLoader(db).Load(roots)
	require.NoError(t, err)
	assert.Equal(t, "caller root value", content.Posts[2].Text)
}
