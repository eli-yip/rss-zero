package weibochat

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const maxAssetBytes int64 = 512 << 20

type AssetStore interface {
	ListAssets(context.Context, string, string, int) ([]Asset, error)
	SaveAssetResult(context.Context, *Asset) error
}

// ObjectStore is implemented by the existing context-aware MinIO file service.
// SaveStreamContext must consume the stream synchronously before returning success.
type ObjectStore interface {
	SaveStreamContext(context.Context, string, io.ReadCloser, int64) error
	AssetsDomain() string
}

// AssetArchiver transfers unfinished resources independently of message checkpoints.
type AssetArchiver struct {
	client *Client
	store  AssetStore
	files  ObjectStore
}

func NewAssetArchiver(client *Client, store AssetStore, files ObjectStore) *AssetArchiver {
	return &AssetArchiver{client: client, store: store, files: files}
}

type ArchiveResult struct {
	Attempted int
	Stored    int
	Failed    int
}

// ArchiveAssets attempts each unfinished asset at most once per invocation.
// Failed downloads remain retryable. maxAssets bounds work, not total archive size.
func (a *AssetArchiver) ArchiveAssets(ctx context.Context, groupID string, maxAssets int) (ArchiveResult, error) {
	var result ArchiveResult
	if !MessageID(groupID).Valid() || maxAssets <= 0 {
		return result, fmt.Errorf("valid group ID and positive maxAssets are required")
	}
	var after string
	var failures []error
	for result.Attempted < maxAssets {
		assets, err := a.store.ListAssets(ctx, groupID, after, min(100, maxAssets-result.Attempted))
		if err != nil {
			return result, errors.Join(append(failures, err)...)
		}
		if len(assets) == 0 {
			break
		}
		for _, asset := range assets {
			result.Attempted++
			err := a.archive(ctx, &asset)
			if err != nil {
				result.Failed++
				asset.Status = AssetFailed
				asset.LastError = err.Error()
				failures = append(failures, fmt.Errorf("asset %s: %w", asset.AssetKey, err))
			} else {
				asset.Status = AssetStored
				asset.LastError = ""
			}
			if saveErr := a.store.SaveAssetResult(ctx, &asset); saveErr != nil {
				return result, errors.Join(append(failures, saveErr)...)
			}
			if err == nil {
				result.Stored++
			}
			after = asset.AssetKey
		}
	}
	return result, errors.Join(failures...)
}

func (a *AssetArchiver) archive(ctx context.Context, asset *Asset) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if !MessageID(asset.GroupID).Valid() || !validAssetKey(asset.AssetKey) {
		return fmt.Errorf("invalid asset identity")
	}
	source := asset.SourceURL
	if fid, ok := strings.CutPrefix(asset.AssetKey, "file/"); ok {
		source = "https://upload.api.weibo.com/2/mss/msget?" + url.Values{"source": {weiboSource}, "fid": {fid}}.Encode()
	}
	stored, err := a.client.storeMedia(ctx, a.files, source, asset.Kind, "weibo-chat/"+asset.GroupID+"/"+asset.AssetKey)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	asset.SourceURL, asset.ObjectKey, asset.StorageProvider = source, stored.key, a.files.AssetsDomain()
	asset.ContentType, asset.ByteSize, asset.StoredAt = stored.contentType, stored.size, &now
	asset.Width, asset.Height = stored.width, stored.height
	asset.DisplayObjectKey = stored.displayKey
	return nil
}

type storedMedia struct {
	key           string
	displayKey    string
	contentType   string
	size          int64
	width, height int
}

// storeMedia is shared by message attachments and user avatars.
func (c *Client) storeMedia(ctx context.Context, files ObjectStore, source string, kind AssetKind, keyPrefix string) (storedMedia, error) {
	response, err := c.download(ctx, source)
	if err != nil {
		return storedMedia{}, err
	}
	defer response.Body.Close()
	if response.ContentLength == 0 {
		return storedMedia{}, fmt.Errorf("empty asset")
	}
	if response.ContentLength > maxAssetBytes {
		return storedMedia{}, fmt.Errorf("asset exceeds size limit")
	}
	prefix, err := io.ReadAll(io.LimitReader(response.Body, 512))
	if err != nil {
		return storedMedia{}, fmt.Errorf("read asset header: %w", err)
	}
	reader := io.MultiReader(bytes.NewReader(prefix), response.Body)
	if (kind == AssetImage || kind == AssetSticker) && isHEIC(prefix) {
		return storeHEIC(ctx, files, reader, response.ContentLength, keyPrefix)
	}
	contentType := http.DetectContentType(prefix)
	ext := assetExtension(kind, contentType)
	if ext == "" {
		return storedMedia{}, fmt.Errorf("unexpected asset content type: %s", contentType)
	}
	width, height := 0, 0
	if kind == AssetImage || kind == AssetSticker {
		var header bytes.Buffer
		width, height, err = ReadImageDimensions(io.TeeReader(reader, &header))
		if err != nil {
			return storedMedia{}, fmt.Errorf("read image dimensions: %w", err)
		}
		reader = io.MultiReader(bytes.NewReader(header.Bytes()), reader)
	}
	stream := &assetStream{reader: reader, body: response.Body, remaining: maxAssetBytes}
	objectKey := keyPrefix + "." + ext
	if err = files.SaveStreamContext(ctx, objectKey, stream, response.ContentLength); err != nil {
		return storedMedia{}, fmt.Errorf("store asset: %w", err)
	}
	if stream.size == 0 || (response.ContentLength >= 0 && stream.size != response.ContentLength) {
		return storedMedia{}, fmt.Errorf("incomplete asset stream")
	}
	return storedMedia{key: objectKey, contentType: contentType, size: stream.size, width: width, height: height}, nil
}

func (c *Client) download(ctx context.Context, source string) (*http.Response, error) {
	parsed, err := url.Parse(source)
	if err != nil || !mediaURLAllowed(parsed) {
		return nil, fmt.Errorf("unsupported media URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", "https://weibo.com")
	if parsed.Hostname() == "upload.api.weibo.com" {
		if err = c.authorize(req); err != nil {
			return nil, err
		}
	}
	client := *c.http
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= 5 || !mediaURLAllowed(next.URL) {
			return fmt.Errorf("unsupported media redirect")
		}
		// A CDN does not need the account's SUB credential.
		if next.URL.Hostname() != "upload.api.weibo.com" {
			next.Header.Del("Cookie")
		}
		return nil
	}
	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download asset: %w", err)
	}
	if response.StatusCode != http.StatusOK {
		_ = response.Body.Close()
		return nil, fmt.Errorf("download asset: HTTP %d", response.StatusCode)
	}
	return response, nil
}

func mediaURLAllowed(u *url.URL) bool {
	if u == nil || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || (u.Port() != "" && u.Port() != "443" && u.Port() != "80") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if host == "upload.api.weibo.com" {
		return u.Scheme == "https"
	}
	return host == "sinaimg.cn" || strings.HasSuffix(host, ".sinaimg.cn") || host == "weibocdn.com" || strings.HasSuffix(host, ".weibocdn.com")
}

func validAssetKey(key string) bool {
	kind, id, ok := strings.Cut(key, "/")
	if !ok {
		return false
	}
	switch kind {
	case "file":
		return MessageID(id).Valid()
	case "pic":
		return safePictureID(id)
	case "url":
		if len(id) != 64 {
			return false
		}
		for _, c := range id {
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				return false
			}
		}
		return true
	}
	return false
}

func assetExtension(kind AssetKind, contentType string) string {
	// Files currently accept PDF bytes only. Upstream names and MIME headers
	// cannot distinguish an attachment from an authentication/error response.
	if kind == AssetFile && contentType == "application/pdf" {
		return "pdf"
	}
	if kind == AssetImage || kind == AssetSticker {
		switch contentType {
		case "image/jpeg":
			return "jpg"
		case "image/png":
			return "png"
		case "image/gif":
			return "gif"
		case "image/webp":
			return "webp"
		}
	}
	if kind == AssetVideo {
		switch contentType {
		case "video/mp4":
			return "mp4"
		case "video/webm":
			return "webm"
		case "video/quicktime":
			return "mov"
		case "video/mpeg":
			return "mpeg"
		}
	}
	return ""
}

// assetStream stops uploads with an error at the limit, rather than storing a
// silently truncated file. It counts bytes without buffering the full attachment.
type assetStream struct {
	reader    io.Reader
	body      io.Closer
	remaining int64
	size      int64
}

func (s *assetStream) Read(p []byte) (int, error) {
	if int64(len(p)) > s.remaining+1 {
		p = p[:s.remaining+1]
	}
	n, err := s.reader.Read(p)
	s.size += int64(n)
	s.remaining -= int64(n)
	if s.remaining < 0 {
		return n, fmt.Errorf("asset exceeds size limit")
	}
	return n, err
}
func (s *assetStream) Close() error { return s.body.Close() }
