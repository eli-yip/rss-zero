package migrate

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/pkg/routers/tombkeeper"
)

var repairPNG = []byte("\x89PNG\r\n\x1a\n" + strings.Repeat("x", 600))

// 仅实现本次迁移使用的操作，其他接口调用应在测试中暴露。
type repairFiles struct {
	file.ContextFile
	objects                   map[string][]byte
	readErr, openErr, saveErr error
	wait                      bool
	closed                    bool
	saves                     int
}

func (f *repairFiles) AssetsDomain() string { return "https://oss.test/rss" }
func (f *repairFiles) GetStreamContext(ctx context.Context, key string) (io.ReadCloser, error) {
	if f.openErr != nil {
		return nil, f.openErr
	}
	if f.wait {
		return &repairReadCloser{read: func([]byte) (int, error) { <-ctx.Done(); return 0, ctx.Err() }, close: func() { f.closed = true }}, nil
	}
	if f.readErr != nil {
		return &repairReadCloser{read: func([]byte) (int, error) { return 0, f.readErr }, close: func() { f.closed = true }}, nil
	}
	b, ok := f.objects[key]
	if !ok {
		return nil, minio.ErrorResponse{Code: "NoSuchKey"}
	}
	return &repairReadCloser{read: bytes.NewReader(b).Read, close: func() { f.closed = true }}, nil
}
func (f *repairFiles) SaveStreamContext(_ context.Context, key string, b io.ReadCloser, _ int64) error {
	defer b.Close()
	if f.saveErr != nil {
		return f.saveErr
	}
	data, err := io.ReadAll(b)
	if err != nil {
		return err
	}
	f.objects[key] = data
	f.saves++
	return nil
}

type repairReadCloser struct {
	read  func([]byte) (int, error)
	close func()
}

func (r *repairReadCloser) Read(b []byte) (int, error) { return r.read(b) }
func (r *repairReadCloser) Close() error               { r.close(); return nil }

type repairMemoryStore struct {
	assets    map[string]tombkeeper.ImageAsset
	updateErr error
	updates   int
}

func (s *repairMemoryStore) ListAfter(after string, limit int) ([]tombkeeper.ImageAsset, error) {
	var ids []string
	for id := range s.assets {
		if id > after {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	if len(ids) > limit {
		ids = ids[:limit]
	}
	var out []tombkeeper.ImageAsset
	for _, id := range ids {
		out = append(out, s.assets[id])
	}
	return out, nil
}
func (s *repairMemoryStore) Update(old, next tombkeeper.ImageAsset) error {
	if s.updateErr != nil {
		return s.updateErr
	}
	s.assets[old.ID] = next
	s.updates++
	return nil
}
func newImageRepairTest(t *testing.T) (*imageRepairer, *repairMemoryStore, *repairFiles, tombkeeper.ImageAsset) {
	t.Helper()
	old := tombkeeper.ImageAsset{ID: "pic", ObjectKey: "tombkeeper/pic.jpg", URL: "https://cdn.ipfsscan.io/weibo/large/pic.jpg", StorageProvider: []string{"https://oss.test/rss"}}
	store := &repairMemoryStore{assets: map[string]tombkeeper.ImageAsset{old.ID: old}}
	f := &repairFiles{objects: map[string][]byte{old.ObjectKey: []byte("<html>Welcome</html>")}}
	worker := &imageRepairer{store: store, files: f, logger: zap.NewNop(), timeout: time.Second,
		download: func(string) (*http.Response, string, error) {
			return &http.Response{Body: io.NopCloser(bytes.NewReader(repairPNG)), ContentLength: int64(len(repairPNG)), Header: http.Header{"Content-Type": []string{"image/png"}}}, "https://wx1.sinaimg.cn/large/pic.jpg", nil
		}}
	return worker, store, f, old
}
func TestIPFSImageSource(t *testing.T) {
	for _, u := range []string{"https://cdn.ipfsscan.io/weibo/a.jpg", "http://CDN.IPFSSCAN.IO:80/a"} {
		require.True(t, isIPFSImageSource(u))
	}
	for _, u := range []string{"https://cdn.ipfsscan.io.evil/a", "https://other/a?cdn.ipfsscan.io/", "ftp://cdn.ipfsscan.io/a", "not a url"} {
		require.False(t, isIPFSImageSource(u))
	}
}
func TestIPFSRepairPreservesOldObjectAndRetriesDatabaseFailure(t *testing.T) {
	worker, store, f, old := newImageRepairTest(t)
	store.updateErr = errors.New("database unavailable")
	_, err := worker.repair(old)
	require.Error(t, err)
	require.Equal(t, old, store.assets[old.ID])
	require.Equal(t, []byte("<html>Welcome</html>"), f.objects[old.ObjectKey])
	store.updateErr = nil
	changed, err := worker.repair(old)
	require.NoError(t, err)
	require.True(t, changed)
	next := store.assets[old.ID]
	require.Equal(t, "tombkeeper/repair-20260903/pic.png", next.ObjectKey)
	require.Equal(t, repairPNG, f.objects[next.ObjectKey])
	require.NotEqual(t, old.URL, next.URL)
	stats, err := runImageRepair(store, zap.NewNop(), worker.repair)
	require.NoError(t, err)
	require.Zero(t, stats.Scanned)
	require.Equal(t, 1, store.updates)
}
func TestIPFSRepairHealthyAndAbandoned(t *testing.T) {
	worker, store, f, old := newImageRepairTest(t)
	f.objects[old.ObjectKey] = repairPNG
	changed, err := worker.repair(old)
	require.NoError(t, err)
	require.False(t, changed)
	require.Zero(t, f.saves)
	require.Zero(t, store.updates)
	old.Status = tombkeeper.ObjectStatusAbandoned
	changed, err = worker.repair(old)
	require.NoError(t, err)
	require.True(t, changed)
	require.Zero(t, f.saves)
	require.Equal(t, old.URL, store.assets[old.ID].URL)
	require.Zero(t, store.assets[old.ID].Status)
}
func TestIPFSRepairMissingAndInvalidObjects(t *testing.T) {
	for _, name := range []string{"html", "empty", "missing", "no key", "lazy missing"} {
		t.Run(name, func(t *testing.T) {
			worker, store, f, old := newImageRepairTest(t)
			switch name {
			case "empty":
				f.objects[old.ObjectKey] = nil
			case "missing":
				delete(f.objects, old.ObjectKey)
			case "no key":
				old.ObjectKey = ""
				old.StorageProvider = nil
			case "lazy missing":
				f.readErr = minio.ErrorResponse{Code: "NoSuchKey"}
			}
			changed, err := worker.repair(old)
			require.NoError(t, err)
			require.True(t, changed)
			require.Equal(t, 1, store.updates)
		})
	}
}
func TestIPFSRepairReadErrorsDoNotOverwrite(t *testing.T) {
	for _, name := range []string{"open denied", "lazy denied", "lazy network", "lazy truncated", "timeout", "unknown provider", "empty provider"} {
		t.Run(name, func(t *testing.T) {
			worker, store, f, old := newImageRepairTest(t)
			switch name {
			case "open denied":
				f.openErr = minio.ErrorResponse{Code: "AccessDenied"}
			case "lazy denied":
				f.readErr = minio.ErrorResponse{Code: "AccessDenied"}
			case "lazy network":
				f.readErr = errors.New("network failure")
			case "lazy truncated":
				f.readErr = io.ErrUnexpectedEOF
			case "timeout":
				f.wait = true
				worker.timeout = time.Millisecond
			case "unknown provider":
				old.StorageProvider = []string{"https://other.test/rss"}
			case "empty provider":
				old.StorageProvider = nil
			}
			_, err := worker.repair(old)
			require.Error(t, err)
			require.Zero(t, f.saves)
			require.Zero(t, store.updates)
			if strings.HasPrefix(name, "lazy") || name == "timeout" {
				require.True(t, f.closed)
			}
		})
	}
}
func TestIPFSRepairDownloadAndUploadFailure(t *testing.T) {
	for _, upload := range []bool{false, true} {
		worker, store, f, old := newImageRepairTest(t)
		if upload {
			f.saveErr = errors.New("upload failed")
		} else {
			worker.download = func(string) (*http.Response, string, error) { return nil, "", errors.New("all sources failed") }
		}
		_, err := worker.repair(old)
		require.Error(t, err)
		require.Zero(t, store.updates)
		require.Equal(t, old, store.assets[old.ID])
	}
}
func TestIPFSRepairContinuesAndPaginates(t *testing.T) {
	_, store, _, old := newImageRepairTest(t)
	store.assets = map[string]tombkeeper.ImageAsset{}
	for i := range 205 {
		a := old
		a.ID = string(rune(1000 + i))
		store.assets[a.ID] = a
	}
	calls := 0
	stats, err := runImageRepair(store, zap.NewNop(), func(tombkeeper.ImageAsset) (bool, error) {
		calls++
		if calls == 1 {
			return false, errors.New("failed")
		}
		return true, nil
	})
	require.Error(t, err)
	require.Equal(t, 205, calls)
	require.Equal(t, 204, stats.Repaired)
	require.Equal(t, 1, stats.Failed)
}
func TestIPFSRepairRegistered(t *testing.T) {
	require.NoError(t, validateRegistry(registry))
	m := registeredMigration(20260903000000)
	require.NotNil(t, m)
	require.True(t, m.Auto)
	require.False(t, m.RequiresPredecessors)
}

func TestIPFSRepairDatabase(t *testing.T) {
	dsn := os.Getenv("TOMBKEEPER_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TOMBKEEPER_TEST_DATABASE_URL for isolated PostgreSQL")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	// 独立 schema 内执行，避免与其他测试的表清理相互影响。
	tx := db.Begin()
	require.NoError(t, tx.Error)
	defer tx.Rollback()
	require.NoError(t, tx.Exec("CREATE SCHEMA image_repair_test").Error)
	require.NoError(t, tx.Exec("SET LOCAL search_path TO image_repair_test").Error)
	require.NoError(t, tx.AutoMigrate(&tombkeeper.ImageAsset{}))
	_, _, _, old := newImageRepairTest(t)
	require.NoError(t, tx.Create(&old).Error)
	deleted := old
	deleted.ID = "deleted"
	require.NoError(t, tx.Create(&deleted).Error)
	require.NoError(t, tx.Delete(&deleted).Error)
	otherType := old
	otherType.ID = "video"
	otherType.Type = 1
	require.NoError(t, tx.Create(&otherType).Error)
	store := imageRepairDB{tx}
	rows, err := store.ListAfter("", 100)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	next := old
	next.URL = "https://wx1.sinaimg.cn/large/pic.jpg"
	next.ObjectKey = "tombkeeper/repair-20260903/pic.png"
	require.NoError(t, store.Update(old, next))
	require.Error(t, store.Update(old, next))
	var got tombkeeper.ImageAsset
	require.NoError(t, tx.First(&got, "id = ?", old.ID).Error)
	require.Equal(t, next.URL, got.URL)
	require.Equal(t, next.ObjectKey, got.ObjectKey)
	require.Equal(t, next.StorageProvider, got.StorageProvider)
	rows, err = store.ListAfter("pic", 100)
	require.NoError(t, err)
	require.Empty(t, rows)
	// 历史列允许 NULL，读取后的 Go 零值仍需匹配原记录。
	require.NoError(t, tx.Exec("UPDATE tombkeeper_object SET object_key = NULL, status = NULL, updated_at = NULL WHERE id = ?", old.ID).Error)
	var legacy tombkeeper.ImageAsset
	require.NoError(t, tx.First(&legacy, "id = ?", old.ID).Error)
	require.NoError(t, store.Update(legacy, next))
}
