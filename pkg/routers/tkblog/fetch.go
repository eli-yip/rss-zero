package tkblog

import (
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

const (
	siteBaseURL = "https://tombkeeper.io"
	userAgent   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// The two blog sources. category is interpolated into the fetch URL, so ValidCategory
// gates it as a trust boundary (also re-checked at the StartCrawl entry point).
const (
	CategoryXfocus = "xfocus"
	CategoryBaidu  = "baidu"
)

// ValidCategory reports whether category is one of the two known blog sources.
func ValidCategory(category string) bool {
	return category == CategoryXfocus || category == CategoryBaidu
}

// One request per reqBaseDelay + up to reqJitter of random jitter (0.8s–1.2s apart).
const (
	reqBaseDelay = 800 * time.Millisecond
	reqJitter    = 400 * time.Millisecond
)

// Requester fetches tombkeeper.io blog list pages. Close stops the rate-limiter
// goroutine and must be called when done (a fresh Requester is created per crawl).
//
// ponytail: tkblog holds its own fetcher rather than sharing tombkeeper's. Same
// host, so the rate limiter could later be unified — kept separate this round to
// avoid touching the golden-tested weibo code.
type Requester interface {
	GetPage(category string, page int) ([]byte, error)
	Close()
}

type RequestService struct {
	client    *http.Client
	limiter   chan struct{}
	done      chan struct{}
	closeOnce sync.Once
	maxRetry  int
	logger    *zap.Logger
}

// redirectGuard caps the hop count and blocks any redirect to a host the allow
// predicate rejects (anti-SSRF).
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

// pageHostAllowed reports whether host is a tombkeeper.io page host — the only host
// GetPage legitimately fetches or follows a redirect to.
func pageHostAllowed(host string) bool {
	host = strings.ToLower(host)
	return host == "tombkeeper.io" || strings.HasSuffix(host, ".tombkeeper.io")
}

func NewRequestService(logger *zap.Logger) Requester {
	rs := &RequestService{
		client: &http.Client{
			Timeout:       30 * time.Second,
			CheckRedirect: redirectGuard(pageHostAllowed),
		},
		limiter:  make(chan struct{}),
		done:     make(chan struct{}),
		maxRetry: 3,
		logger:   logger,
	}
	go rs.runLimiter()
	return rs
}

// runLimiter feeds the limiter one token per base+rand(jitter) interval until done.
func (rs *RequestService) runLimiter() {
	for {
		select {
		case rs.limiter <- struct{}{}:
		case <-rs.done:
			return
		}
		select {
		case <-time.After(reqBaseDelay + rand.N(reqJitter)):
		case <-rs.done:
			return
		}
	}
}

// Close stops the rate-limiter goroutine. Safe to call multiple times.
func (rs *RequestService) Close() { rs.closeOnce.Do(func() { close(rs.done) }) }

// GetPage fetches one blog list page. Page 1 is the bare /{category} URL; page N≥2
// is /{category}?page=N (page 1 must NOT carry ?page=1, or the site may serve a
// redirect/404 and the totalPages read would be wrong).
func (rs *RequestService) GetPage(category string, page int) ([]byte, error) {
	if !ValidCategory(category) {
		return nil, fmt.Errorf("invalid category: %q", category)
	}
	u := siteBaseURL + "/" + category
	if page >= 2 {
		u = fmt.Sprintf("%s?page=%d", u, page)
	}
	return rs.getHTML(u)
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
