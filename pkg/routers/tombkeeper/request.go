package tombkeeper

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	siteBaseURL = "https://tombkeeper.io"
	userAgent   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	weiboRefer  = "https://weibo.com"
)

// getHTML requests share one global rate limiter: one request per reqBaseDelay plus
// up to reqJitter of random jitter (i.e. 0.5s–1.0s apart), regardless of domain.
const (
	reqBaseDelay = 500 * time.Millisecond
	reqJitter    = 500 * time.Millisecond
	// Image fetches use a separate, faster limiter: one image per picBaseDelay
	// plus up to picJitter jitter (0.25s–0.5s apart). The token is taken once per
	// image (before the per-image CDN fan-out), so the rate of distinct images is
	// bounded while intra-image candidate probing stays concurrent.
	picBaseDelay = 250 * time.Millisecond
	picJitter    = 250 * time.Millisecond
)

// Requester fetches tombkeeper.io pages and weibo images. tombkeeper.io needs no
// cookie; images need a weibo Referer to bypass sinaimg hotlink protection.
// Close stops the rate-limiter goroutine and must be called when done (a fresh
// Requester is created per crawl run).
type Requester interface {
	GetPage(page int) ([]byte, error)
	GetPageRange(startDate, endDate string, page int) ([]byte, error)
	GetDetail(id string) ([]byte, error)
	GetReppic(longURL string) (picIDs []string, err error)
	GetPicStream(ctx context.Context, url string) (*http.Response, error)
	WaitPicSlot()
	Close()
}

type RequestService struct {
	client     *http.Client
	picClient  *http.Client
	limiter    chan struct{}
	picLimiter chan struct{}
	done       chan struct{}
	closeOnce  sync.Once
	maxRetry   int
	logger     *zap.Logger
}

// redirectGuard returns a CheckRedirect that caps the hop count and blocks any
// redirect to a host the allow predicate rejects (anti-SSRF).
func redirectGuard(allow func(host string) bool) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		if !allow(req.URL.Hostname()) {
			return fmt.Errorf("redirect to disallowed host: %s", req.URL.Hostname())
		}
		return nil
	}
}

// pageHostAllowed reports whether host is a tombkeeper.io or weibo page host —
// the only hosts GetPage/GetDetail/GetReppic legitimately fetch or follow a
// redirect to.
func pageHostAllowed(host string) bool {
	host = strings.ToLower(host)
	if host == "tombkeeper.io" || host == "photo.weibo.com" {
		return true
	}
	return strings.HasSuffix(host, ".tombkeeper.io") || strings.HasSuffix(host, ".weibo.com")
}

func NewRequestService(logger *zap.Logger) Requester {
	rs := &RequestService{
		// Defense-in-depth: never follow a redirect to a host outside the relevant
		// allowlist. The page client (tombkeeper.io list/detail and the
		// photo.weibo.com H5 page fetched by GetReppic) and the image client each
		// get their own host check, so an upstream page or CDN cannot 302 a
		// server-side fetch to internal infra.
		client: &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: redirectGuard(pageHostAllowed),
		},
		picClient: &http.Client{
			Timeout:       20 * time.Second,
			CheckRedirect: redirectGuard(hostAllowedHost),
		},
		limiter:    make(chan struct{}),
		picLimiter: make(chan struct{}),
		done:       make(chan struct{}),
		maxRetry:   3,
		logger:     logger,
	}
	go rs.runLimiter(rs.limiter, reqBaseDelay, reqJitter)
	go rs.runLimiter(rs.picLimiter, picBaseDelay, picJitter)
	return rs
}

// runLimiter feeds ch one token per base+rand(jitter) interval until done.
func (rs *RequestService) runLimiter(ch chan struct{}, base, jitter time.Duration) {
	for {
		select {
		case ch <- struct{}{}:
		case <-rs.done:
			return
		}
		select {
		case <-time.After(base + rand.N(jitter)):
		case <-rs.done:
			return
		}
	}
}

// WaitPicSlot blocks until the image rate limiter releases a token. Callers take
// one token per image (before the CDN fan-out), bounding the image-fetch rate
// while keeping intra-image candidate probing concurrent.
func (rs *RequestService) WaitPicSlot() { <-rs.picLimiter }

// Close stops the rate-limiter goroutine. Safe to call multiple times.
func (rs *RequestService) Close() {
	rs.closeOnce.Do(func() { close(rs.done) })
}

func (rs *RequestService) GetPage(page int) ([]byte, error) {
	return rs.getHTML(fmt.Sprintf("%s/?page=%d", siteBaseURL, page))
}

// GetPageRange fetches one list page restricted to the [startDate, endDate]
// window (both YYYY-MM-DD; the site returns posts newest-first, page 1 nearest
// endDate). The date strings are validated by the caller, so they need no
// escaping.
func (rs *RequestService) GetPageRange(startDate, endDate string, page int) ([]byte, error) {
	return rs.getHTML(fmt.Sprintf("%s/?startDate=%s&endDate=%s&page=%d", siteBaseURL, startDate, endDate, page))
}

func (rs *RequestService) GetDetail(id string) ([]byte, error) {
	return rs.getHTML(fmt.Sprintf("%s/weibo/%s", siteBaseURL, id))
}

// reppicSinaRe matches the sinaimg image URLs embedded in a photo.weibo.com repost
// image H5 page; group 1 is the bare pic id.
var reppicSinaRe = regexp.MustCompile(`sinaimg\.cn/[a-z0-9_]+/([0-9a-zA-Z]+)\.(?:jpg|jpeg|gif|png|webp)`)

// GetReppic resolves a "查看图片" repost-image H5 page (a 带图转发 reposter's own
// attached image) to the bare sinaimg pic ids it carries, in page order, de-duped.
// The page is server-rendered, so the sinaimg URLs are present in the HTML. The host
// is checked against photo.weibo.com (anti-SSRF). Returns an error if the page can't
// be fetched; an empty slice if it carries no sinaimg image.
func (rs *RequestService) GetReppic(longURL string) ([]string, error) {
	u, err := url.Parse(longURL)
	if err != nil {
		return nil, fmt.Errorf("parse reppic url: %w", err)
	}
	if !strings.EqualFold(u.Hostname(), "photo.weibo.com") {
		return nil, fmt.Errorf("reppic host not allowed: %s", u.Hostname())
	}
	html, err := rs.getHTML(longURL)
	if err != nil {
		return nil, err
	}
	return extractReppicPicIDs(html), nil
}

// extractReppicPicIDs pulls the bare sinaimg pic ids out of a repost image H5 page.
func extractReppicPicIDs(html []byte) []string {
	var ids []string
	seen := make(map[string]struct{})
	for _, m := range reppicSinaRe.FindAllSubmatch(html, -1) {
		id := string(m[1])
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids
}

func (rs *RequestService) getHTML(u string) (data []byte, err error) {
	logger := rs.logger.With(zap.String("url", u))
	for i := range rs.maxRetry {
		<-rs.limiter

		req, e := http.NewRequest(http.MethodGet, u, nil)
		if e != nil {
			err = e
			continue
		}
		req.Header.Set("User-Agent", userAgent)

		resp, e := rs.client.Do(req)
		if e != nil {
			err = e
			logger.Warn("request failed", zap.Int("attempt", i), zap.Error(e))
			continue
		}
		body, e := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if e != nil {
			err = e
			continue
		}
		if resp.StatusCode != http.StatusOK {
			err = fmt.Errorf("bad status code: %d", resp.StatusCode)
			continue
		}
		return body, nil
	}
	return nil, fmt.Errorf("failed to get %s: %w", u, err)
}

// GetPicStream downloads an image with a weibo Referer; the caller owns resp.Body.
// ctx lets the caller abort an in-flight probe once another candidate has won.
func (rs *RequestService) GetPicStream(ctx context.Context, picURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, picURL, nil)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Referer", weiboRefer)
	req.Header.Set("User-Agent", userAgent)

	resp, err := rs.picClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("bad status code: %d", resp.StatusCode)
	}
	// Some third-party image proxies (notably image.baidu.com/search/down) answer a
	// missing image with "200 OK, Content-Length: 0" — an empty success. Reject it
	// so it never wins the candidate race in downloadFirstAvailable; the genuine
	// sinaimg variant (non-empty) wins instead. Without this guard the empty body is
	// streamed to OSS as a 0-byte file and recorded ObjectStatusOK, leaving the post
	// body embedding a working OSS URL that serves a broken (0-byte) image.
	if resp.ContentLength == 0 {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("empty image body (content-length 0): %s", picURL)
	}
	return resp, nil
}
