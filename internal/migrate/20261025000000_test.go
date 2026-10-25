package migrate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/weibochat"
)

func TestWeiboDimensionsBackfillRetriesOnlyMissingDimensions(t *testing.T) {
	dsn := os.Getenv("WEIBOCHAT_TEST_DSN")
	if dsn == "" {
		t.Skip("set WEIBOCHAT_TEST_DSN for PostgreSQL integration tests")
	}
	base, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	schema := fmt.Sprintf("image_dimensions_test_%d", time.Now().UnixNano())
	require.NoError(t, base.Exec("CREATE SCHEMA "+schema).Error)
	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		conn, _ := db.DB()
		_ = conn.Close()
		_ = base.Exec("DROP SCHEMA " + schema + " CASCADE").Error
		conn, _ = base.DB()
		_ = conn.Close()
	})
	require.NoError(t, db.AutoMigrate(&weibochat.Asset{}))
	assets := []weibochat.Asset{
		{GroupID: "1", AssetKey: "pic/a", Kind: "image", Status: weibochat.AssetStored, ObjectKey: "a.png"},
		{GroupID: "1", AssetKey: "pic/b", Kind: "sticker", Status: weibochat.AssetStored, ObjectKey: "b.png"},
		{GroupID: "2", AssetKey: "pic/a", Kind: "image", Status: weibochat.AssetStored, ObjectKey: "c.png", Width: 7, Height: 9},
		{GroupID: "2", AssetKey: "file/1", Kind: "video", Status: weibochat.AssetStored, ObjectKey: "a.mp4"},
	}
	require.NoError(t, db.Create(&assets).Error)
	var data bytes.Buffer
	require.NoError(t, png.Encode(&data, image.NewNRGBA(image.Rect(0, 0, 32, 24))))
	calls := map[string]int{}
	broken := true
	open := func(_ context.Context, a weibochat.Asset) (io.ReadCloser, error) {
		calls[a.ObjectKey]++
		if a.ObjectKey == "b.png" && broken {
			return nil, errors.New("temporary storage failure")
		}
		return io.NopCloser(bytes.NewReader(data.Bytes())), nil
	}
	require.Error(t, backfillWeiboImageDimensions(db, zap.NewNop(), open))
	broken = false
	require.NoError(t, backfillWeiboImageDimensions(db, zap.NewNop(), open))
	require.NoError(t, backfillWeiboImageDimensions(db, zap.NewNop(), open))
	require.Equal(t, map[string]int{"a.png": 1, "b.png": 2}, calls)
	var saved weibochat.Asset
	require.NoError(t, db.Where("group_id = ? AND asset_key = ?", "1", "pic/b").First(&saved).Error)
	require.Equal(t, 32, saved.Width)
	require.Equal(t, 24, saved.Height)
	require.Equal(t, weibochat.AssetStored, saved.Status)
	require.True(t, registeredMigration(20261025000000).Auto)
}
