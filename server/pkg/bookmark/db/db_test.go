package db

import (
	"testing"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/db"

	"github.com/stretchr/testify/assert"
)

func TestBookmarkDB(t *testing.T) {
	assert := assert.New(t)
	_ = config.InitForTestToml()
	postgresDB, err := db.NewPostgresDB(config.C.Database)
	assert.Nil(err)
	dbService := &BookmarkDBImpl{postgresDB}
	t.Run("TestSingleBookmark", func(t *testing.T) {
		b, err := dbService.NewBookmark("jason", 0, "test single bookmarks")
		assert.Nil(err)
		_, err = dbService.AddTag(b.ID, "test tag1")
		assert.Nil(err)
		_, err = dbService.AddTag(b.ID, "test tag2")
		assert.Nil(err)
		ts, err := dbService.GetTag(b.ID)
		assert.Nil(err)
		assert.Len(ts, 2)
		err = dbService.RemoveTag(b.ID, "test tag1")
		assert.Nil(err)
		ts, err = dbService.GetTag(b.ID)
		assert.Nil(err)
		assert.Len(ts, 1)

		// clean up
		err = dbService.RemoveTag(b.ID, "test tag2")
		assert.Nil(err)
		err = dbService.RemoveBookmark(b.ID)
		assert.Nil(err)
	})

	t.Run("TestMultiBookmark", func(t *testing.T) {
		b1, err := dbService.NewBookmark("jason", 0, "test multi bookmark1")
		assert.Nil(err)
		b2, err := dbService.NewBookmark("jason", 0, "test multi bookmark2")
		assert.Nil(err)

		_, err = dbService.AddTag(b1.ID, "test tag1")
		assert.Nil(err)
		_, err = dbService.AddTag(b1.ID, "test tag2")
		assert.Nil(err)
		_, err = dbService.AddTag(b2.ID, "test tag1")
		assert.Nil(err)
		_, err = dbService.AddTag(b2.ID, "test tag2")
		assert.Nil(err)

		bs, err := dbService.GetBookmarkByTags("jason", []string{"test tag1", "test tag2"})
		assert.Nil(err)
		assert.Len(bs, 2)

		bs, err = dbService.GetBookmarkByTag("jason", "test tag1")
		assert.Nil(err)
		assert.Len(bs, 2)
		err = dbService.RemoveTag(b1.ID, "test tag1")
		assert.Nil(err)
		bs, err = dbService.GetBookmarkByTag("jason", "test tag1")
		assert.Nil(err)
		assert.Len(bs, 1)
		bs, err = dbService.GetBookmarkByTags("jason", []string{"test tag1", "test tag2"})
		assert.Nil(err)
		assert.Len(bs, 2)
		err = dbService.RemoveTag(b2.ID, "test tag1")
		assert.Nil(err)
		bs, err = dbService.GetBookmarkByTag("jason", "test tag1")
		assert.Nil(err)
		assert.Len(bs, 0)

		err = dbService.RemoveBookmark(b1.ID)
		assert.Nil(err)
		ts, err := dbService.GetTag(b1.ID)
		assert.Nil(err)
		assert.Len(ts, 0)

		// clean up
		err = dbService.RemoveBookmark(b1.ID)
		assert.Nil(err)
		err = dbService.RemoveBookmark(b2.ID)
		assert.Nil(err)
	})

	t.Run("TestDeleteBookmark", func(t *testing.T) {
		b1, err := dbService.NewBookmark("jason", 0, "test multi bookmark1")
		assert.Nil(err)

		err = dbService.RemoveBookmark(b1.ID)
		assert.Nil(err)

		err = dbService.RemoveBookmark(b1.ID)
		assert.Nil(err)
	})

	t.Run("TestDeleteTag", func(t *testing.T) {
		b1, err := dbService.NewBookmark("jason", 0, "test multi bookmark1")
		assert.Nil(err)

		_, err = dbService.AddTag(b1.ID, "test tag1")
		assert.Nil(err)

		err = dbService.RemoveTag(b1.ID, "test tag1")
		assert.Nil(err)

		err = dbService.RemoveTag(b1.ID, "test tag1")
		assert.Nil(err)

		err = dbService.RemoveBookmark(b1.ID)
		assert.Nil(err)

		err = dbService.RemoveBookmark(b1.ID)
		assert.Nil(err)
	})

	t.Run("TestGetBookmarkByUser", func(t *testing.T) {
		t.Run("SingleBookmark", func(t *testing.T) {
			b1, err := dbService.NewBookmark("jason", 0, "test content")
			assert.Nil(err)

			_, err = dbService.AddTag(b1.ID, "tag1")
			assert.Nil(err)
			_, err = dbService.AddTag(b1.ID, "tag2")
			assert.Nil(err)
			_, err = dbService.AddTag(b1.ID, "tag3")
			assert.Nil(err)

			t.Run("Include", func(t *testing.T) {
				bs, err := dbService.GetBookmarkByUser("jason", &BookmarkQuery{
					Tag: &TagFilter{
						Include: []string{"tag1", "tag2"},
					},
				})
				assert.Nil(err)
				assert.Len(bs, 1)
			})

			t.Run("IncludeMore", func(t *testing.T) {
				bs, err := dbService.GetBookmarkByUser("jason", &BookmarkQuery{
					Tag: &TagFilter{
						Include: []string{"tag1", "tag2", "tag4"},
					},
				})
				assert.Nil(err)
				assert.Len(bs, 1)
			})

			t.Run("Exclude", func(t *testing.T) {
				bs, err := dbService.GetBookmarkByUser("jason", &BookmarkQuery{
					Tag: &TagFilter{
						Exclude: []string{"tag1"},
					},
				})
				assert.Nil(err)
				assert.Len(bs, 0)
			})

			t.Run("NoTag", func(t *testing.T) {
				bs, err := dbService.GetBookmarkByUser("jason", &BookmarkQuery{
					Tag: &TagFilter{
						NoTag: true,
					},
				})
				assert.Nil(err)
				assert.Len(bs, 0)
			})

			t.Run("DeleteTagInclude", func(t *testing.T) {
				err = dbService.RemoveTag(b1.ID, "tag1")
				assert.Nil(err)

				bs, err := dbService.GetBookmarkByUser("jason", &BookmarkQuery{
					Tag: &TagFilter{
						Include: []string{"tag1"},
					},
				})
				assert.Nil(err)
				assert.Len(bs, 0)

				_, err = dbService.AddTag(b1.ID, "tag1")
				assert.Nil(err)
			})

			t.Run("DeleteTagExclude", func(t *testing.T) {
				err = dbService.RemoveTag(b1.ID, "tag1")
				assert.Nil(err)

				bs, err := dbService.GetBookmarkByUser("jason", &BookmarkQuery{
					Tag: &TagFilter{
						Exclude: []string{"tag1"},
					},
				})
				assert.Nil(err)
				assert.Len(bs, 1)

				_, err = dbService.AddTag(b1.ID, "tag1")
				assert.Nil(err)
			})

			err = dbService.RemoveBookmark(b1.ID)
			assert.Nil(err)
		})

		t.Run("MultiBookmark", func(t *testing.T) {
			b1, err := dbService.NewBookmark("jason", 0, "content 1")
			assert.Nil(err)
			_, err = dbService.AddTag(b1.ID, "tag1")
			assert.Nil(err)
			_, err = dbService.AddTag(b1.ID, "tag2")
			assert.Nil(err)

			b2, err := dbService.NewBookmark("jason", 0, "content 2")
			assert.Nil(err)
			_, err = dbService.AddTag(b2.ID, "tag2")
			assert.Nil(err)
			_, err = dbService.AddTag(b2.ID, "tag3")
			assert.Nil(err)

			b3, err := dbService.NewBookmark("jason", 0, "content 3")
			assert.Nil(err)

			t.Run("IncludeSingleTag", func(t *testing.T) {
				bs, err := dbService.GetBookmarkByUser("jason", &BookmarkQuery{
					Tag: &TagFilter{
						Include: []string{"tag1"},
					},
				})
				assert.Nil(err)
				assert.Len(bs, 1)
			})

			t.Run("IncludeMultipleTags", func(t *testing.T) {
				bs, err := dbService.GetBookmarkByUser("jason", &BookmarkQuery{
					Tag: &TagFilter{
						Include: []string{"tag1", "tag3"},
					},
				})
				assert.Nil(err)
				assert.Len(bs, 2)
			})

			t.Run("ExcludeSingleTag", func(t *testing.T) {
				bs, err := dbService.GetBookmarkByUser("jason", &BookmarkQuery{
					Tag: &TagFilter{
						Exclude: []string{"tag1"},
					},
				})
				assert.Nil(err)
				assert.Len(bs, 2)
			})

			t.Run("ExcludeMultipleTags", func(t *testing.T) {
				bs, err := dbService.GetBookmarkByUser("jason", &BookmarkQuery{
					Tag: &TagFilter{
						Exclude: []string{"tag1", "tag3"},
					},
				})
				assert.Nil(err)
				assert.Len(bs, 1)
			})

			t.Run("NoTag", func(t *testing.T) {
				bs, err := dbService.GetBookmarkByUser("jason", &BookmarkQuery{
					Tag: &TagFilter{
						NoTag: true,
					},
				})
				assert.Nil(err)
				assert.Len(bs, 1)
			})

			t.Run("IncludeAndExclude", func(t *testing.T) {
				bs, err := dbService.GetBookmarkByUser("jason", &BookmarkQuery{
					Tag: &TagFilter{
						Include: []string{"tag2"},
						Exclude: []string{"tag1"},
					},
				})
				assert.Nil(err)
				assert.Len(bs, 1)
			})

			err = dbService.RemoveBookmark(b1.ID)
			assert.Nil(err)
			err = dbService.RemoveBookmark(b2.ID)
			assert.Nil(err)
			err = dbService.RemoveBookmark(b3.ID)
			assert.Nil(err)
		})
	})

	t.Run("TestGetTag", func(t *testing.T) {
		b1, err := dbService.NewBookmark("jason", 0, "test get tag")
		assert.Nil(err)
		b2, err := dbService.NewBookmark("jason", 0, "test get tag")
		assert.Nil(err)
		defer dbService.RemoveBookmark(b1.ID)
		defer dbService.RemoveBookmark(b2.ID)

		_, err = dbService.AddTag(b1.ID, "tag1")
		assert.Nil(err)
		_, err = dbService.AddTag(b1.ID, "tag2")
		assert.Nil(err)

		t.Run("Test single bookmark", func(t *testing.T) {
			b1tags, err := dbService.GetTag(b1.ID)
			assert.Nil(err)
			assert.Len(b1tags, 2)

			b2tags, err := dbService.GetTag(b2.ID)
			assert.Nil(err)
			assert.Len(b2tags, 0)
			assert.NotNil(b2tags)
		})

		t.Run("Test multi bookmark", func(t *testing.T) {
			tagMap, err := dbService.GetTags([]string{b1.ID, b2.ID})
			assert.Nil(err)

			v, ok := tagMap[b1.ID]
			assert.True(ok)
			assert.Len(v, 2)
			v, ok = tagMap[b2.ID]
			assert.False(ok)
			assert.Nil(v)
		})
	})

	t.Run("GetTagCountByUser", func(t *testing.T) {
		b1, err := dbService.NewBookmark("jason", 0, "test get tag count")
		assert.Nil(err)
		b2, err := dbService.NewBookmark("jason", 0, "test get tag count")
		assert.Nil(err)
		defer dbService.RemoveBookmark(b1.ID)
		defer dbService.RemoveBookmark(b2.ID)

		_, err = dbService.AddTag(b1.ID, "tag1")
		assert.Nil(err)
		_, err = dbService.AddTag(b1.ID, "tag2")
		assert.Nil(err)
		_, err = dbService.AddTag(b2.ID, "tag2")
		assert.Nil(err)
		_, err = dbService.AddTag(b2.ID, "tag3")
		assert.Nil(err)

		t.Run("Common case", func(t *testing.T) {
			tagCount, err := dbService.GetTagCountByUser("jason")
			assert.Nil(err)
			assert.Len(tagCount, 3)
			assert.Equal(tagCount[0].Name, "tag2")
			assert.Equal(tagCount[0].Count, 2)
		})

		t.Run("No tags", func(t *testing.T) {
			err = dbService.RemoveTag(b1.ID, "tag1")
			assert.Nil(err)
			err = dbService.RemoveTag(b1.ID, "tag2")
			assert.Nil(err)
			err = dbService.RemoveTag(b2.ID, "tag2")
			assert.Nil(err)
			err = dbService.RemoveTag(b2.ID, "tag3")
			assert.Nil(err)

			tagCount, err := dbService.GetTagCountByUser("jason")
			assert.Nil(err)
			assert.Len(tagCount, 0)
		})
	})
}
