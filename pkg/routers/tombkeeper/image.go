package tombkeeper

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/internal/file"
)

var errAllCDNFailed = errors.New("all cdn candidates failed")

type imageRequester interface {
	GetPicStream(ctx context.Context, url string) (*http.Response, error)
	WaitPicSlot()
}

type imageAssetStore interface {
	SaveImageAsset(asset *ImageAsset) error
	ImageAssetExists(id string) (bool, error)
}

// archiveImageAsset 归档源图片并记录归档结果，不生成展示文本。
func archiveImageAsset(req imageRequester, f file.File, store imageAssetStore, picField string, logger *zap.Logger) error {
	picID := picIDOf(picField)
	if picID == "" {
		return fmt.Errorf("empty pic id from %q", picField)
	}

	exists, err := store.ImageAssetExists(picID)
	if err != nil {
		return fmt.Errorf("check image asset %q: %w", picID, err)
	}
	if exists {
		return nil
	}

	cands, originalLink := candidateURLs(picField)
	if len(cands) == 0 {
		// 保留非白名单源链接，但不从服务端请求。
		return nil
	}
	resp, usedURL, derr := downloadFirstAvailable(req, cands)
	if derr != nil {
		logger.Warn("all CDNs failed, abandoning image", zap.String("pic_id", picID))
		if err := store.SaveImageAsset(&ImageAsset{
			ID: picID, Type: ObjectTypeImage,
			URL: originalLink, Status: ObjectStatusAbandoned,
		}); err != nil {
			return fmt.Errorf("save abandoned image asset: %w", err)
		}
		return nil
	}
	// SaveStream 负责关闭 resp.Body。
	ext := extFromContentType(resp.Header.Get("Content-Type"))
	objectKey := fmt.Sprintf("tombkeeper/%s.%s", picID, ext)
	if err := f.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
		// 持久化失败结果，避免后续导入重复下载。
		logger.Warn("oss save failed, abandoning image", zap.String("pic_id", picID), zap.Error(err))
		if saveErr := store.SaveImageAsset(&ImageAsset{
			ID: picID, Type: ObjectTypeImage,
			URL: originalLink, Status: ObjectStatusAbandoned,
		}); saveErr != nil {
			return fmt.Errorf("save abandoned image asset: %w", saveErr)
		}
		return nil
	}

	asset := &ImageAsset{
		ID: picID, Type: ObjectTypeImage,
		ObjectKey: objectKey, URL: usedURL,
		StorageProvider: []string{f.AssetsDomain()}, Status: ObjectStatusOK,
	}
	if err := store.SaveImageAsset(asset); err != nil {
		return fmt.Errorf("save image asset: %w", err)
	}
	return nil
}

// RedownloadObject 为零字节回填原地修复已有图片资产。
func RedownloadObject(req Requester, f file.File, objectKey, picID string, logger *zap.Logger) (usedURL string, err error) {
	cands, _ := candidateURLs(picID)
	if len(cands) == 0 {
		return "", fmt.Errorf("no download candidates for pic %q", picID)
	}
	resp, usedURL, err := downloadFirstAvailable(req, cands)
	if err != nil {
		return "", fmt.Errorf("download pic %q: %w", picID, err)
	}
	// SaveStream 负责关闭 resp.Body。
	if err = f.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
		return "", fmt.Errorf("save pic %q to %q: %w", picID, objectKey, err)
	}
	logger.Info("redownloaded image",
		zap.String("pic_id", picID), zap.String("object_key", objectKey), zap.String("used_url", usedURL))
	return usedURL, nil
}

// candidateURLs expands a pics entry into download candidates, plus the original
// link to keep if every candidate fails.
func candidateURLs(picField string) (cands []string, originalLink string) {
	picField = strings.TrimSpace(picField)
	if strings.HasPrefix(picField, "http") {
		// A full-URL pics entry comes verbatim from the mirror; only fetch it if
		// its host is allowlisted (anti-SSRF). Otherwise keep it as the original
		// link but do not issue a server-side request to an arbitrary host.
		if !imageURLAllowed(picField) {
			return nil, picField
		}
		return append([]string{picField}, thirdPartyProxies(picField)...), picField
	}
	sina := sinaCandidates(picIDOf(picField))
	return append(sina, thirdPartyProxies(sina[0])...), sina[0]
}

// hostAllowedHost reports whether host is a sinaimg CDN or one of the known
// third-party image proxies. Used to constrain server-side image fetches and
// redirect targets (anti-SSRF).
func hostAllowedHost(host string) bool {
	host = strings.ToLower(host)
	if host == "sinaimg.cn" || strings.HasSuffix(host, ".sinaimg.cn") {
		return true
	}
	switch host {
	case "cdn.cdnjson.com", "i0.wp.com", "cdn.ipfsscan.io", "image.baidu.com":
		return true
	}
	return false
}

func imageURLAllowed(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return false
	}
	return hostAllowedHost(u.Hostname())
}

// sinaCandidates builds the sinaimg host variants (wx/ww/tvax x 1-4) for a pic id.
func sinaCandidates(picID string) []string {
	out := make([]string, 0, 12)
	for _, host := range []string{"wx", "ww", "tvax"} {
		for i := range 4 {
			out = append(out, fmt.Sprintf("https://%s%d.sinaimg.cn/large/%s.jpg", host, i+1, picID))
		}
	}
	return out
}

// thirdPartyProxies mirrors image-seeker's fallback proxies derived from a sina URL.
func thirdPartyProxies(sinaURL string) []string {
	noProto := strings.TrimPrefix(strings.TrimPrefix(sinaURL, "https://"), "http://")
	pathOnly := noProto
	if i := strings.Index(noProto, "/"); i >= 0 {
		pathOnly = noProto[i:]
	}
	return []string{
		"https://image.baidu.com/search/down?url=" + sinaURL,
		"https://cdn.cdnjson.com/" + noProto,
		"https://i0.wp.com/" + noProto,
		"https://cdn.ipfsscan.io/weibo" + pathOnly,
	}
}

// picIDOf extracts the bare pic id (without extension) from a pics entry.
func picIDOf(picField string) string {
	picField = strings.TrimSpace(picField)
	if picField == "" {
		return ""
	}
	base := picField
	if strings.Contains(picField, "/") {
		if u, err := url.Parse(picField); err == nil {
			base = path.Base(u.Path)
		} else {
			base = picField[strings.LastIndex(picField, "/")+1:]
		}
	}
	if dot := strings.LastIndex(base, "."); dot > 0 {
		base = base[:dot]
	}
	return base
}

func extFromContentType(ct string) string {
	ct = strings.ToLower(ct)
	switch {
	case strings.Contains(ct, "gif"):
		return "gif"
	case strings.Contains(ct, "png"):
		return "png"
	case strings.Contains(ct, "webp"):
		return "webp"
	default:
		return "jpg"
	}
}

// downloadFirstAvailable probes all candidates concurrently (GET with weibo
// Referer, via Requester) and returns the first successful response. The losing
// probes are cancelled the instant a winner is chosen — rather than left running
// to completion or the client timeout — and any straggler that succeeded in the
// meantime has its body drained and closed. The winner keeps its own context
// alive until the caller closes its body (see cancelOnClose). Returns
// errAllCDNFailed if none succeed.
func downloadFirstAvailable(req imageRequester, cands []string) (*http.Response, string, error) {
	req.WaitPicSlot() // throttle the image-fetch rate; the fan-out below stays concurrent

	type result struct {
		resp *http.Response
		url  string
		idx  int
	}
	results := make(chan result, len(cands))
	cancels := make([]context.CancelFunc, len(cands))
	var wg sync.WaitGroup
	for i, u := range cands {
		ctx, cancel := context.WithCancel(context.Background())
		cancels[i] = cancel
		wg.Add(1)
		go func(i int, u string) {
			defer wg.Done()
			resp, err := req.GetPicStream(ctx, u)
			if err != nil {
				cancel() // release this probe's context on failure/cancellation
				return
			}
			results <- result{resp, u, i}
		}(i, u)
	}
	go func() { wg.Wait(); close(results) }()

	winner, ok := <-results
	if !ok {
		return nil, "", errAllCDNFailed
	}
	// Abort every other in-flight probe immediately; keep the winner's context
	// alive until the caller closes its body.
	for i, c := range cancels {
		if i != winner.idx {
			c()
		}
	}
	go func() {
		for s := range results { // drain & close any straggler that beat the cancel
			_ = s.resp.Body.Close()
		}
	}()
	winner.resp.Body = &cancelOnClose{ReadCloser: winner.resp.Body, cancel: cancels[winner.idx]}
	return winner.resp, winner.url, nil
}

// cancelOnClose cancels the request context when the response body is closed, so
// the winning probe's connection is released only after its body is consumed.
type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	c.cancel()
	return c.ReadCloser.Close()
}
