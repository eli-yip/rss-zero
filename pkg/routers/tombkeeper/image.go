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

// saveImage rehosts one pic (a bare pic id or a full sinaimg URL) to OSS and
// returns the markdown image URL to embed. On total CDN failure it records an
// abandoned object and returns the original sinaimg link, so the body still
// references the image. It is idempotent: an already-recorded object is reused.
func saveImage(req Requester, f file.File, db DB, postID int64, picField string, logger *zap.Logger) (markdownURL string, err error) {
	picID := picIDOf(picField)
	if picID == "" {
		return "", fmt.Errorf("empty pic id from %q", picField)
	}

	if exists, _ := db.ObjectExists(picID); exists {
		if o, e := db.GetObject(picID); e == nil {
			return objectMarkdownURL(o), nil
		}
	}

	cands, originalLink := candidateURLs(picField)
	resp, usedURL, derr := downloadFirstAvailable(req, cands)
	if derr != nil {
		logger.Warn("all CDNs failed, keeping original link", zap.String("pic_id", picID))
		_ = db.SaveObject(&Object{
			ID: picID, PostID: postID, Type: ObjectTypeImage,
			URL: originalLink, Status: ObjectStatusAbandoned,
		})
		return originalLink, nil
	}
	// resp.Body is handed to SaveStream, which closes it.
	ext := extFromContentType(resp.Header.Get("Content-Type"))
	objectKey := fmt.Sprintf("tombkeeper/%s.%s", picID, ext)
	if err = f.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
		// OSS write failed: degrade like total-CDN-failure rather than dropping the
		// image. Record it abandoned (keeping the original link) so the body still
		// shows the image and the next crawl skips it instead of re-downloading.
		logger.Warn("oss save failed, keeping original link", zap.String("pic_id", picID), zap.Error(err))
		_ = db.SaveObject(&Object{
			ID: picID, PostID: postID, Type: ObjectTypeImage,
			URL: originalLink, Status: ObjectStatusAbandoned,
		})
		return originalLink, nil
	}

	obj := &Object{
		ID: picID, PostID: postID, Type: ObjectTypeImage,
		ObjectKey: objectKey, URL: usedURL,
		StorageProvider: []string{f.AssetsDomain()}, Status: ObjectStatusOK,
	}
	if err = db.SaveObject(obj); err != nil {
		return "", fmt.Errorf("save object: %w", err)
	}
	return obj.URI()
}

// objectMarkdownURL returns the OSS URI for a stored object, or its original link
// when the object was abandoned.
func objectMarkdownURL(o *Object) string {
	if o.Status == ObjectStatusOK {
		if uri, err := o.URI(); err == nil {
			return uri
		}
	}
	return o.URL
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
func downloadFirstAvailable(req Requester, cands []string) (*http.Response, string, error) {
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
