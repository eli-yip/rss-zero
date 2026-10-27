package weibochat

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"io"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type memoryAssets struct {
	items    map[string]Asset
	failSave bool
}

func (s *memoryAssets) ListAssets(_ context.Context, group, after string, limit int) ([]Asset, error) {
	var keys []string
	for key, asset := range s.items {
		if asset.GroupID == group && key > after && asset.Status != AssetStored {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var assets []Asset
	for _, key := range keys[:min(len(keys), limit)] {
		assets = append(assets, s.items[key])
	}
	return assets, nil
}
func (s *memoryAssets) SaveAssetResult(_ context.Context, asset *Asset) error {
	if s.failSave {
		return errors.New("database unavailable")
	}
	s.items[asset.AssetKey] = *asset
	return nil
}

type memoryObjects struct {
	keys    []string
	objects map[string][]byte
	fail    bool
}

func (f *memoryObjects) AssetsDomain() string { return "https://assets.example" }
func (f *memoryObjects) SaveStreamContext(_ context.Context, key string, body io.ReadCloser, _ int64) error {
	defer body.Close()
	f.keys = append(f.keys, key)
	if f.fail {
		return errors.New("upload failed")
	}
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	if f.objects == nil {
		f.objects = map[string][]byte{}
	}
	f.objects[key] = data
	return nil
}
func testPNG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 2, 2))))
	return buf.Bytes()
}

func TestArchiveRetriesFailedDownload(t *testing.T) {
	ctx := context.Background()
	valid := false
	calls := 0
	data := testPNG(t)
	client := newTestClient(&http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		require.Equal(t, "/2/mss/msget", req.URL.Path)
		require.Equal(t, "123", req.URL.Query().Get("fid"))
		require.Equal(t, chatReferer, req.Referer())
		cookie, err := req.Cookie("SUB")
		require.NoError(t, err)
		require.Equal(t, "test", cookie.Value)
		payload := []byte("<html>login</html>")
		if valid {
			payload = data
		}
		return &http.Response{StatusCode: 200, ContentLength: int64(len(payload)), Body: io.NopCloser(bytes.NewReader(payload))}, nil
	})}, func(context.Context) (string, error) { return "test", nil })
	store := &memoryAssets{items: map[string]Asset{"file/123": {GroupID: "1", AssetKey: "file/123", Kind: "image", Status: AssetPending}}}
	files := &memoryObjects{}
	archiver := NewAssetArchiver(client, store, files)
	result, err := archiver.ArchiveAssets(ctx, "1", 10)
	require.Error(t, err)
	require.Equal(t, 1, result.Failed)
	require.Equal(t, 1, calls)
	require.Empty(t, files.keys)
	require.Equal(t, AssetFailed, store.items["file/123"].Status)
	valid = true
	result, err = archiver.ArchiveAssets(ctx, "1", 10)
	require.NoError(t, err)
	require.Equal(t, 1, result.Stored)
	saved := store.items["file/123"]
	require.Equal(t, "weibo-chat/1/file/123.png", saved.ObjectKey)
	require.Equal(t, data, files.objects[saved.ObjectKey])
	require.Empty(t, saved.LastError)
	require.EqualValues(t, len(data), saved.ByteSize)
	require.Equal(t, 2, saved.Width)
	require.Equal(t, 2, saved.Height)
	result, err = archiver.ArchiveAssets(ctx, "1", 10)
	require.NoError(t, err)
	require.Zero(t, result.Attempted)
	require.Equal(t, 2, calls)
}

func TestArchiveRecoversUploadAndDatabaseFailures(t *testing.T) {
	for _, failure := range []string{"upload", "database"} {
		t.Run(failure, func(t *testing.T) {
			data := testPNG(t)
			client := newTestClient(&http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, ContentLength: -1, Body: io.NopCloser(bytes.NewReader(data))}, nil
			})}, nil)
			store := &memoryAssets{items: map[string]Asset{"pic/abc": {GroupID: "1", AssetKey: "pic/abc", Kind: "sticker", SourceURL: "https://wx1.sinaimg.cn/large/abc.jpg", Status: AssetPending}}}
			files := &memoryObjects{fail: failure == "upload"}
			store.failSave = failure == "database"
			archiver := NewAssetArchiver(client, store, files)
			_, err := archiver.ArchiveAssets(context.Background(), "1", 10)
			require.Error(t, err)
			require.NotEqual(t, AssetStored, store.items["pic/abc"].Status)
			files.fail = false
			store.failSave = false
			result, err := archiver.ArchiveAssets(context.Background(), "1", 10)
			require.NoError(t, err)
			require.Equal(t, 1, result.Stored)
			require.Equal(t, []string{"weibo-chat/1/pic/abc.png", "weibo-chat/1/pic/abc.png"}, files.keys)
		})
	}
}

func TestDownloadRedirectDoesNotForwardCookie(t *testing.T) {
	calls := 0
	client := newTestClient(&http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"https://wx1.sinaimg.cn/large/abc.jpg"}}, Body: io.NopCloser(strings.NewReader(""))}, nil
		}
		require.Empty(t, req.Header.Get("Cookie"))
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("image"))}, nil
	})}, func(context.Context) (string, error) { return "secret", nil })
	response, err := client.download(context.Background(), "https://upload.api.weibo.com/2/mss/msget?fid=123")
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	require.Equal(t, 2, calls)
	_, err = client.download(context.Background(), "https://example.com/file")
	require.Error(t, err)
	require.Equal(t, 2, calls)
}

func TestAssetSizeLimitFailsRatherThanTruncating(t *testing.T) {
	stream := &assetStream{reader: strings.NewReader("12345"), body: io.NopCloser(strings.NewReader("")), remaining: 4}
	_, err := io.ReadAll(stream)
	require.ErrorContains(t, err, "size limit")
}

func TestArchiveVideoAndTruncatedImage(t *testing.T) {
	// ISO BMFF ftyp header used by Go's standard MIME detector.
	video := []byte{0, 0, 0, 24, 'f', 't', 'y', 'p', 'm', 'p', '4', '2', 0, 0, 0, 0, 'm', 'p', '4', '2', 'i', 's', 'o', 'm'}
	require.Equal(t, "video/mp4", http.DetectContentType(video))
	for _, kind := range []string{"video", "truncated"} {
		t.Run(kind, func(t *testing.T) {
			data := video
			length := int64(len(data))
			assetKind := AssetVideo
			if kind == "truncated" {
				data = testPNG(t)
				length = int64(len(data) + 10)
				assetKind = AssetImage
			}
			client := newTestClient(&http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: 200, ContentLength: length, Body: io.NopCloser(bytes.NewReader(data))}, nil
			})}, func(context.Context) (string, error) { return "test", nil })
			store := &memoryAssets{items: map[string]Asset{"file/123": {GroupID: "1", AssetKey: "file/123", Kind: assetKind, Status: AssetPending}}}
			result, err := NewAssetArchiver(client, store, &memoryObjects{}).ArchiveAssets(context.Background(), "1", 1)
			if kind == "truncated" {
				require.Error(t, err)
				require.Equal(t, 1, result.Failed)
			} else {
				require.NoError(t, err)
				require.Equal(t, 1, result.Stored)
				require.Equal(t, "weibo-chat/1/file/123.mp4", store.items["file/123"].ObjectKey)
			}
		})
	}
}

func TestArchiveCoreIntegration(t *testing.T) {
	store, db := testStore(t)
	ctx := context.Background()
	latest := false
	data := testPNG(t)
	client := newTestClient(&http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
		payload := data
		if req.URL.Host == "api.weibo.com" {
			payload = []byte(`{"result":true,"messages":[{"gid":1,"id":5,"from_uid":42,"from_user":{"screen_name":"first","avatar_large":"https://wx1.sinaimg.cn/avatar/first.png"},"media_type":1,"fids":[123]}]}`)
			if latest {
				payload = []byte(`{"result":true,"messages":[{"gid":1,"id":5,"from_uid":42,"from_user":{"screen_name":"updated","avatar_large":"https://wx1.sinaimg.cn/avatar/updated.png"},"media_type":1,"fids":[123]},{"gid":1,"id":6,"media_type":1,"fids":[124]}]}`)
			}
		}
		return &http.Response{StatusCode: 200, ContentLength: int64(len(payload)), Body: io.NopCloser(bytes.NewReader(payload))}, nil
	})}, func(context.Context) (string, error) { return "test", nil })
	files := &memoryObjects{}
	archiver := NewArchiver(client, store, NewUserArchiver(client, store, files))
	_, err := archiver.Initialize(ctx, "1")
	require.NoError(t, err)
	firstUser, err := store.GetUser(ctx, "42")
	require.NoError(t, err)
	require.Equal(t, data, files.objects[firstUser.AvatarObjectKey])
	latest = true
	synced, err := archiver.Sync(ctx, "1", 2)
	require.NoError(t, err)
	require.Equal(t, 1, synced.MessagesSaved)
	require.Equal(t, MessageID("6"), synced.Through)
	user, err := store.GetUser(ctx, "42")
	require.NoError(t, err)
	require.Equal(t, "updated", user.ScreenName)
	require.Equal(t, "https://wx1.sinaimg.cn/avatar/updated.png", user.AvatarSourceURL)
	require.NotEqual(t, firstUser.AvatarObjectKey, user.AvatarObjectKey)
	require.Equal(t, data, files.objects[user.AvatarObjectKey])
	result, err := NewAssetArchiver(client, store, files).ArchiveAssets(ctx, "1", 10)
	require.NoError(t, err)
	require.Equal(t, 2, result.Stored)
	var assets []Asset
	require.NoError(t, db.Find(&assets).Error)
	require.Len(t, assets, 2)
	for _, asset := range assets {
		require.Equal(t, AssetStored, asset.Status)
		require.Contains(t, asset.SourceURL, "/mss/msget?")
		require.Equal(t, "image/png", asset.ContentType)
		require.Equal(t, data, files.objects[asset.ObjectKey])
	}
}

func TestArchivePDFRejectsErrorResponsesAndRetries(t *testing.T) {
	pdf := []byte("%PDF-1.7\n1 0 obj\n<<>>\nendobj\n%%EOF\n")
	for _, payload := range []string{`{"error":"authentication required"}`, `<html>login</html>`, "PK\x03\x04unsupported zip"} {
		t.Run(payload[:2], func(t *testing.T) {
			data := []byte(payload)
			client := newTestClient(&http.Client{Transport: transportFunc(func(req *http.Request) (*http.Response, error) {
				require.Equal(t, "5358051221832241", req.URL.Query().Get("fid"))
				cookie, err := req.Cookie("SUB")
				require.NoError(t, err)
				require.Equal(t, "test", cookie.Value)
				// MIME metadata is deliberately untrusted.
				return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/pdf"}}, ContentLength: -1, Body: io.NopCloser(bytes.NewReader(data))}, nil
			})}, func(context.Context) (string, error) { return "test", nil })
			store := &memoryAssets{items: map[string]Asset{"file/5358051221832241": {GroupID: "4761715839862414", AssetKey: "file/5358051221832241", Kind: AssetFile, Status: AssetPending}}}
			files := &memoryObjects{}
			archiver := NewAssetArchiver(client, store, files)
			result, err := archiver.ArchiveAssets(t.Context(), "4761715839862414", 10)
			require.ErrorContains(t, err, "unexpected asset content type")
			require.Equal(t, 1, result.Failed)
			require.Empty(t, files.keys)
			require.Equal(t, AssetFailed, store.items["file/5358051221832241"].Status)
			data = pdf
			result, err = archiver.ArchiveAssets(t.Context(), "4761715839862414", 10)
			require.NoError(t, err)
			require.Equal(t, 1, result.Stored)
			saved := store.items["file/5358051221832241"]
			require.Equal(t, "weibo-chat/4761715839862414/file/5358051221832241.pdf", saved.ObjectKey)
			require.Equal(t, "application/pdf", saved.ContentType)
			require.EqualValues(t, len(pdf), saved.ByteSize)
			require.Equal(t, pdf, files.objects[saved.ObjectKey])
			require.Zero(t, saved.Width)
			require.Zero(t, saved.Height)
			require.Empty(t, saved.LastError)
		})
	}
}
