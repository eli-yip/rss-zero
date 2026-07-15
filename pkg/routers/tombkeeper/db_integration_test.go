package tombkeeper

import (
	"os"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDBServiceMonotonicPostUpsert(t *testing.T) {
	db := openTombkeeperIntegrationTestDB(t)
	require.NoError(t, db.AutoMigrate(&Post{}, &ImageAsset{}))
	store := &DBService{DB: db}

	const h5URL = "https://photo.weibo.com/h5"
	base := Post{ID: 1, PublishedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	start := make(chan struct{})
	var wg sync.WaitGroup
	for index := range 20 {
		wg.Go(func() {
			<-start
			post := base
			if index%2 == 0 {
				post.InTimeline = true
				post.H5ImageIDsByURL = map[string][]string{h5URL: {}}
			} else {
				post.H5ImageIDsByURL = map[string][]string{h5URL: {"pic"}}
			}
			if err := store.UpsertPost(&post); err != nil {
				t.Errorf("upsert: %v", err)
			}
		})
	}
	close(start)
	wg.Wait()

	post, err := store.GetPost(1)
	require.NoError(t, err)
	assert.True(t, post.InTimeline)
	assert.Equal(t, []string{"pic"}, post.H5ImageIDsByURL[h5URL])

	newerReference := Post{ID: 2, PublishedAt: base.PublishedAt.Add(time.Hour), InTimeline: false}
	require.NoError(t, store.UpsertPost(&newerReference))
	entries, err := store.LatestTimelineEntries(10)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.EqualValues(t, 1, entries[0].ID)

	start = make(chan struct{})
	for index := range 20 {
		wg.Go(func() {
			<-start
			asset := ImageAsset{ID: "pic", Status: ObjectStatusAbandoned, URL: "failed"}
			if index%2 == 0 {
				asset.Status = ObjectStatusOK
				asset.URL = "source"
				asset.ObjectKey = "tombkeeper/pic.jpg"
				asset.StorageProvider = []string{"https://oss.test"}
			}
			if err := store.SaveImageAsset(&asset); err != nil {
				t.Errorf("save image asset: %v", err)
			}
		})
	}
	close(start)
	wg.Wait()
	asset, err := store.GetImageAsset("pic")
	require.NoError(t, err)
	assert.Equal(t, ObjectStatusOK, asset.Status)
	assert.Equal(t, "tombkeeper/pic.jpg", asset.ObjectKey)
}

func TestTimelineImporterPersistsSuccessfulEmptyH5ImageIndexAsArray(t *testing.T) {
	db := openTombkeeperIntegrationTestDB(t)
	require.NoError(t, db.AutoMigrate(&Post{}, &ImageAsset{}))
	store := &DBService{DB: db}

	html, repost, longURL := viewPicTimelinePage(t)
	req := &fakeRequester{
		reppics:     map[string][]string{longURL: {}},
		reppicCalls: map[string]int{},
	}
	importer := NewTimelineImporter(req, newFakeFile(), store, testLogger())

	stats, err := importer.Import(html)
	require.NoError(t, err)
	assert.Equal(t, ImportStats{EntriesSeen: 1, EntriesSaved: 1}, stats)

	var persisted string
	require.NoError(t, db.Raw(
		"SELECT view_pics -> ? FROM tombkeeper_post WHERE id = ?", longURL, repost.ID,
	).Scan(&persisted).Error)
	assert.JSONEq(t, "[]", persisted)

	post, err := store.GetPost(repost.ID)
	require.NoError(t, err)
	require.NoError(t, store.UpsertPost(post))

	stats, err = importer.Import(html)
	require.NoError(t, err)
	assert.Equal(t, ImportStats{EntriesSeen: 1}, stats)
	assert.Equal(t, 1, req.reppicCalls[longURL])
}

func openTombkeeperIntegrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TOMBKEEPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TOMBKEEPER_TEST_DATABASE_URL to run the Postgres integration test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS tombkeeper_post").Error)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS tombkeeper_object").Error)
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE IF EXISTS tombkeeper_post").Error
		_ = db.Exec("DROP TABLE IF EXISTS tombkeeper_object").Error
	})
	return db
}
