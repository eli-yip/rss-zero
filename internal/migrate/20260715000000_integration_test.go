package migrate

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/pkg/routers/tombkeeper"
)

func TestTombkeeperH5ImageIndexMigrationRepairsNullBeforeUpsert(t *testing.T) {
	db := openTombkeeperH5MigrationTestDB(t)
	require.NoError(t, db.AutoMigrate(&tombkeeper.Post{}))
	store := &tombkeeper.DBService{DB: db}

	const h5URL = "https://photo.weibo.com/h5/repost/reppic_id/test"
	post := tombkeeper.Post{
		ID:              1,
		PublishedAt:     time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC),
		H5ImageIDsByURL: map[string][]string{h5URL: {}},
		InTimeline:      true,
	}
	require.NoError(t, store.UpsertPost(&post))
	require.NoError(t, db.Exec(
		"UPDATE tombkeeper_post SET view_pics = jsonb_build_object(CAST(? AS text), 'null'::jsonb) WHERE id = ?",
		h5URL, post.ID,
	).Error)

	require.NoError(t, migrateTombkeeperH5ImageIndexInvariant(db, zap.NewNop()))

	var persisted string
	require.NoError(t, db.Raw(
		"SELECT view_pics -> ? FROM tombkeeper_post WHERE id = ?", h5URL, post.ID,
	).Scan(&persisted).Error)
	assert.JSONEq(t, "[]", persisted)

	stored, err := store.GetPost(post.ID)
	require.NoError(t, err)
	require.NoError(t, store.UpsertPost(stored))
}

func TestTombkeeperH5ImageIndexMigrationEnforcesDatabaseInvariant(t *testing.T) {
	db := openTombkeeperH5MigrationTestDB(t)
	require.NoError(t, db.AutoMigrate(&tombkeeper.Post{}))
	require.NoError(t, migrateTombkeeperH5ImageIndexInvariant(db, zap.NewNop()))

	assert.True(t, db.Migrator().HasConstraint(
		"tombkeeper_post", "tombkeeper_post_view_pics_arrays",
	))
	invalidValues := []struct {
		name     string
		viewPics any
	}{
		{"SQL null", nil},
		{"top-level array", `[]`},
		{"nested JSON null", `{"url": null}`},
		{"nested string", `{"url": "bad"}`},
		{"nested number", `{"url": 1}`},
		{"nested boolean", `{"url": true}`},
		{"nested object", `{"url": {}}`},
	}
	for index, test := range invalidValues {
		t.Run(test.name, func(t *testing.T) {
			err := db.Exec(
				"INSERT INTO tombkeeper_post (id, published_at, view_pics) VALUES (?, CURRENT_TIMESTAMP, CAST(? AS jsonb))",
				100+index, test.viewPics,
			).Error
			require.Error(t, err)
		})
	}

	require.NoError(t, db.Exec(
		"INSERT INTO tombkeeper_post (id, published_at) VALUES (?, CURRENT_TIMESTAMP)", 200,
	).Error)
	var persisted string
	require.NoError(t, db.Raw(
		"SELECT view_pics FROM tombkeeper_post WHERE id = ?", 200,
	).Scan(&persisted).Error)
	assert.JSONEq(t, "{}", persisted)
}

func TestTombkeeperH5ImageIndexConstraintRejectsInvalidUpsert(t *testing.T) {
	db := openTombkeeperH5MigrationTestDB(t)
	require.NoError(t, db.AutoMigrate(&tombkeeper.Post{}))
	require.NoError(t, migrateTombkeeperH5ImageIndexInvariant(db, zap.NewNop()))
	store := &tombkeeper.DBService{DB: db}

	const h5URL = "https://photo.weibo.com/h5/repost/reppic_id/test"
	existing := tombkeeper.Post{
		ID:              1,
		PublishedAt:     time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC),
		Text:            "existing",
		H5ImageIDsByURL: map[string][]string{h5URL: {"pic"}},
		InTimeline:      true,
	}
	require.NoError(t, store.UpsertPost(&existing))

	incoming := existing
	incoming.Text = "invalid incoming"
	incoming.H5ImageIDsByURL = map[string][]string{h5URL: nil}
	err := store.UpsertPost(&incoming)
	require.ErrorContains(t, err, tombkeeperH5ImageIndexConstraint)

	stored, err := store.GetPost(existing.ID)
	require.NoError(t, err)
	assert.Equal(t, "existing", stored.Text)
	assert.Equal(t, []string{"pic"}, stored.H5ImageIDsByURL[h5URL])
	assert.True(t, stored.InTimeline)
}

func TestTombkeeperH5ImageIndexMigrationPreservesMonotonicUpsert(t *testing.T) {
	db := openTombkeeperH5MigrationTestDB(t)
	require.NoError(t, db.AutoMigrate(&tombkeeper.Post{}))
	require.NoError(t, migrateTombkeeperH5ImageIndexInvariant(db, zap.NewNop()))
	store := &tombkeeper.DBService{DB: db}

	const h5URL = "https://photo.weibo.com/h5/repost/reppic_id/test"
	base := tombkeeper.Post{
		ID:              1,
		PublishedAt:     time.Date(2026, 7, 15, 0, 0, 0, 0, time.UTC),
		H5ImageIDsByURL: map[string][]string{h5URL: {}},
	}
	require.NoError(t, store.UpsertPost(&base))

	withImage := base
	withImage.H5ImageIDsByURL = map[string][]string{h5URL: {"pic"}}
	require.NoError(t, store.UpsertPost(&withImage))

	emptyTimelineEntry := base
	emptyTimelineEntry.InTimeline = true
	require.NoError(t, store.UpsertPost(&emptyTimelineEntry))

	emptyReference := base
	emptyReference.InTimeline = false
	require.NoError(t, store.UpsertPost(&emptyReference))

	stored, err := store.GetPost(base.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{"pic"}, stored.H5ImageIDsByURL[h5URL])
	assert.True(t, stored.InTimeline)
}

func TestTombkeeperH5ImageIndexMigrationNormalizesLegacyShapes(t *testing.T) {
	db := openTombkeeperH5MigrationTestDB(t)
	require.NoError(t, db.AutoMigrate(&tombkeeper.Post{}))
	const seed = `INSERT INTO tombkeeper_post (id, published_at, view_pics) VALUES
	(1, CURRENT_TIMESTAMP, NULL),
	(2, CURRENT_TIMESTAMP, 'null'::jsonb),
	(3, CURRENT_TIMESTAMP, '"bad"'::jsonb),
	(4, CURRENT_TIMESTAMP, '1'::jsonb),
	(5, CURRENT_TIMESTAMP, 'true'::jsonb),
	(6, CURRENT_TIMESTAMP, '[]'::jsonb),
	(7, CURRENT_TIMESTAMP, '{
		"null": null,
		"string": "bad",
		"number": 1,
		"boolean": true,
		"object": {},
		"empty": [],
		"nonempty": ["pic"]
	}'::jsonb)`
	require.NoError(t, db.Exec(seed).Error)

	require.NoError(t, migrateTombkeeperH5ImageIndexInvariant(db, zap.NewNop()))
	require.NoError(t, migrateTombkeeperH5ImageIndexInvariant(db, zap.NewNop()))

	for id := 1; id <= 6; id++ {
		var persisted string
		require.NoError(t, db.Raw(
			"SELECT view_pics FROM tombkeeper_post WHERE id = ?", id,
		).Scan(&persisted).Error)
		assert.JSONEq(t, "{}", persisted, "id=%d", id)
	}
	var object string
	require.NoError(t, db.Raw(
		"SELECT view_pics FROM tombkeeper_post WHERE id = ?", 7,
	).Scan(&object).Error)
	assert.JSONEq(t, `{
		"null": [],
		"string": [],
		"number": [],
		"boolean": [],
		"object": [],
		"empty": [],
		"nonempty": ["pic"]
	}`, object)
}

func TestTombkeeperH5ImageIndexMigrationOnFreshSchema(t *testing.T) {
	db := openTombkeeperH5MigrationTestDB(t)
	require.NoError(t, db.AutoMigrate(&tombkeeper.Post{}))
	require.NoError(t, migrateTombkeeperH5ImageIndexInvariant(db, zap.NewNop()))
	require.NoError(t, migrateTombkeeperH5ImageIndexInvariant(db, zap.NewNop()))
	assert.True(t, db.Migrator().HasConstraint(
		"tombkeeper_post", tombkeeperH5ImageIndexConstraint,
	))
}

func openTombkeeperH5MigrationTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TOMBKEEPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TOMBKEEPER_TEST_DATABASE_URL to run the Postgres integration test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.Exec("DROP TABLE IF EXISTS tombkeeper_post").Error)
	t.Cleanup(func() { _ = db.Exec("DROP TABLE IF EXISTS tombkeeper_post").Error })
	return db
}
