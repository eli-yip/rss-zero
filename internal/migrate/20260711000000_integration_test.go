package migrate

import (
	"errors"
	"io"
	"os"
	"testing"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/eli-yip/rss-zero/pkg/routers/tombkeeper"
)

func TestTombkeeperStructuredContentMigrationDoesNotTruncateOnRetry(t *testing.T) {
	dsn := os.Getenv("TOMBKEEPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TOMBKEEPER_TEST_DATABASE_URL to run the Postgres integration test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	for _, statement := range []string{
		"DROP TABLE IF EXISTS tombkeeper_post",
		"DROP TABLE IF EXISTS tombkeeper_object",
		"CREATE TABLE tombkeeper_post (id bigint PRIMARY KEY, text_markdown text, title text, video_url text, raw bytea, created_at timestamptz, retweet_id text)",
		"CREATE TABLE tombkeeper_object (id text PRIMARY KEY, post_id bigint)",
		"INSERT INTO tombkeeper_post (id, text_markdown) VALUES (1, 'legacy')",
		"CREATE VIEW tombkeeper_legacy_view AS SELECT text_markdown FROM tombkeeper_post",
	} {
		require.NoError(t, db.Exec(statement).Error)
	}
	t.Cleanup(func() {
		_ = db.Exec("DROP TABLE IF EXISTS tombkeeper_post").Error
		_ = db.Exec("DROP TABLE IF EXISTS tombkeeper_object").Error
	})
	require.NoError(t, db.AutoMigrate(&tombkeeper.Post{}, &tombkeeper.ImageAsset{}))

	require.Error(t, migrateTombkeeperStructuredContent(db, zap.NewNop()))
	var legacyCount int64
	require.NoError(t, db.Table("tombkeeper_post").Where("id = ?", 1).Count(&legacyCount).Error)
	assert.EqualValues(t, 1, legacyCount)
	assert.True(t, db.Migrator().HasColumn(&legacyTombkeeperPost{}, "text_markdown"))
	require.NoError(t, db.Exec("DROP VIEW tombkeeper_legacy_view").Error)

	require.NoError(t, migrateTombkeeperStructuredContent(db, zap.NewNop()))
	post := tombkeeper.Post{ID: 2, Text: "new", InTimeline: true}
	require.NoError(t, (&tombkeeper.DBService{DB: db}).UpsertPost(&post))

	// 模拟迁移已提交但记账失败后的重试。
	require.NoError(t, migrateTombkeeperStructuredContent(db, zap.NewNop()))
	var count int64
	require.NoError(t, db.Model(&tombkeeper.Post{}).Where("id = ?", 2).Count(&count).Error)
	assert.EqualValues(t, 1, count)
	for _, column := range []string{"title", "text_markdown", "video_url", "raw", "created_at", "retweet_id"} {
		assert.False(t, db.Migrator().HasColumn("tombkeeper_post", column), column)
	}
	assert.False(t, db.Migrator().HasColumn("tombkeeper_object", "post_id"))
}

func TestTombkeeperStructuredContentMigrationOnFreshSchema(t *testing.T) {
	dsn := os.Getenv("TOMBKEEPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TOMBKEEPER_TEST_DATABASE_URL to run the Postgres integration test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { _ = tx.Rollback().Error })

	require.NoError(t, MigrateDB(tx))
	RunAuto(tx, zap.NewNop(), nil)
	applied, err := loadApplied(tx)
	require.NoError(t, err)
	assert.True(t, applied.Contains(int64(20260711000000)))
	assert.True(t, tx.Migrator().HasColumn(&tombkeeper.Post{}, "in_timeline"))
	assert.False(t, tx.Migrator().HasColumn(&legacyTombkeeperPost{}, "text_markdown"))
}

func TestTombkeeperLivePageOneAfterFreshMigration(t *testing.T) {
	if os.Getenv("TOMBKEEPER_LIVE_TEST") != "1" {
		t.Skip("set TOMBKEEPER_LIVE_TEST=1 to run the live page smoke test")
	}
	dsn := os.Getenv("TOMBKEEPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TOMBKEEPER_TEST_DATABASE_URL to run the Postgres integration test")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	tx := db.Begin()
	require.NoError(t, tx.Error)
	t.Cleanup(func() { _ = tx.Rollback().Error })
	require.NoError(t, MigrateDB(tx))
	RunAuto(tx, zap.NewNop(), nil)

	req := tombkeeper.NewRequestService(zap.NewNop())
	defer req.Close()
	html, err := req.GetPage(1)
	require.NoError(t, err)
	store := &tombkeeper.DBService{DB: tx}
	stats, err := tombkeeper.NewTimelineImporter(req, discardImageFile{}, store, zap.NewNop()).Import(html)
	require.NoError(t, err)
	assert.Positive(t, stats.EntriesSeen)
	assert.Positive(t, stats.EntriesSaved)
	entries, err := store.LatestTimelineEntries(1)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.True(t, entries[0].InTimeline)
}

type discardImageFile struct{}

func (discardImageFile) SaveStream(_ string, stream io.ReadCloser, _ int64) error {
	defer stream.Close()
	_, err := io.Copy(io.Discard, stream)
	return err
}

func (discardImageFile) GetStream(string) (io.ReadCloser, error) {
	return nil, errors.New("unsupported")
}

func (discardImageFile) AssetsDomain() string       { return "https://assets.test" }
func (discardImageFile) Delete(string) error        { return nil }
func (discardImageFile) Exist(string) (bool, error) { return false, nil }
func (discardImageFile) Size(string) (int64, error) { return 0, nil }
