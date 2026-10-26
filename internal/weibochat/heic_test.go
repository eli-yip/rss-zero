package weibochat

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gen2brain/h265/heic"
	"github.com/stretchr/testify/require"
)

func testHEIC(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 32, 24))
	for y := range 24 {
		for x := range 32 {
			img.Set(x, y, color.NRGBA{R: 220, G: 40, B: 20, A: 255})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, heic.Encode(&buf, img, heic.EncodeOptions{Quality: 90}))
	return buf.Bytes()
}

type displayFailObjects struct {
	memoryObjects
	failDisplay bool
}

func (f *displayFailObjects) SaveStreamContext(ctx context.Context, key string, body io.ReadCloser, size int64) error {
	if f.failDisplay && strings.HasSuffix(key, ".display.jpg") {
		_ = body.Close()
		return errors.New("display upload failed")
	}
	return f.memoryObjects.SaveStreamContext(ctx, key, body, size)
}

func TestHEICArchiveRetriesBothObjects(t *testing.T) {
	data := testHEIC(t)
	require.True(t, isHEIC(data[:min(len(data), 512)]))
	client := NewClient(&http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, ContentLength: int64(len(data)), Header: http.Header{"Content-Type": {"application/octet-stream;charset=utf-8"}}, Body: io.NopCloser(bytes.NewReader(data))}, nil
	})}, func(context.Context) (string, error) { return "test", nil })
	store := &memoryAssets{items: map[string]Asset{"file/123": {GroupID: "1", AssetKey: "file/123", Kind: AssetImage, Status: AssetFailed}}}
	files := &displayFailObjects{failDisplay: true}
	archive := NewAssetArchiver(client, store, files)
	result, err := archive.ArchiveAssets(t.Context(), "1", 10)
	require.ErrorContains(t, err, "display upload failed")
	require.Equal(t, 1, result.Failed)
	require.Equal(t, AssetFailed, store.items["file/123"].Status)
	require.Empty(t, store.items["file/123"].DisplayObjectKey)
	files.failDisplay = false
	result, err = archive.ArchiveAssets(t.Context(), "1", 10)
	require.NoError(t, err)
	require.Equal(t, 1, result.Stored)
	saved := store.items["file/123"]
	require.Equal(t, "image/heic", saved.ContentType)
	require.EqualValues(t, len(data), saved.ByteSize)
	require.Equal(t, data, files.objects[saved.ObjectKey])
	display, err := jpeg.Decode(bytes.NewReader(files.objects[saved.DisplayObjectKey]))
	require.NoError(t, err)
	require.Equal(t, image.Rect(0, 0, 32, 24), display.Bounds())
	require.Equal(t, 32, saved.Width)
	require.Equal(t, 24, saved.Height)
	require.Empty(t, saved.LastError)
}

func TestHEICRejectsInvalidAndTruncatedDownloads(t *testing.T) {
	data := testHEIC(t)
	for _, payload := range [][]byte{data[:24], data} {
		files := &memoryObjects{}
		_, err := storeHEIC(t.Context(), files, bytes.NewReader(payload), int64(len(data)+1), "test")
		require.ErrorContains(t, err, "incomplete asset stream")
		require.Empty(t, files.keys)
	}
	files := &memoryObjects{}
	_, err := storeHEIC(t.Context(), files, bytes.NewReader(data[:24]), 24, "test")
	require.Error(t, err)
	require.Empty(t, files.keys)
	require.False(t, isHEIC([]byte("<html>login</html>")))
	require.False(t, isHEIC([]byte("\x00\x00\x00\x18ftypavif\x00\x00\x00\x00mif1avif")))
}
