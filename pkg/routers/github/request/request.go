package request

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	maxLoggedResponseBodyBytes = 64 * 1024
	userAPIURL                 = "https://api.github.com/user"
	defaultRequestTimeout      = 15 * time.Second
)

var requestTimeout = defaultRequestTimeout

type Release struct {
	ID          int       `json:"id"`
	HTMLURL     string    `json:"html_url"`
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	CreatedAt   time.Time `json:"created_at"`
	PublishedAt time.Time `json:"published_at"`
}

var (
	ErrNoRelease = errors.New("releases API response is empty")
	// ErrUnauthorized is returned on HTTP 401 (bad credentials). 403 is deliberately
	// excluded — GitHub also uses it for rate limiting, where the token is still valid.
	ErrUnauthorized = errors.New("github token unauthorized")
)

// APIError 保留 GitHub 返回的有限诊断信息，不包含 Authorization 等敏感请求头。
type APIError struct {
	StatusCode         int
	URL                string
	RequestID          string
	RateLimitRemaining string
	RateLimitReset     string
	RateLimitResource  string
	RetryAfter         string
	Body               string
	BodyReadError      string
}

func (e *APIError) Error() string {
	return fmt.Sprintf(
		"github API request failed: status=%d url=%q request_id=%q rate_limit_remaining=%q rate_limit_reset=%q rate_limit_resource=%q retry_after=%q response_body=%s response_body_read_error=%q",
		e.StatusCode, e.URL, e.RequestID, e.RateLimitRemaining, e.RateLimitReset, e.RateLimitResource, e.RetryAfter, e.Body, e.BodyReadError,
	)
}

func (e *APIError) Unwrap() error {
	if e.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	return nil
}

func GetRepoReleases(user, repo, token string) (releases []Release, err error) {
	releases = make([]Release, 0)

	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	resp, err := get(ctx, fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", user, repo), token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, responseError(resp)
	}

	if err = json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode releases API response: %w", err)
	}

	if len(releases) == 0 {
		return nil, ErrNoRelease
	}

	return releases, nil
}

// ValidateToken 通过用户端点独立验证 token，避免把单个仓库请求的一次 401
// 直接解释为 token 永久失效。
func ValidateToken(token string) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	resp, err := get(ctx, userAPIURL, token)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return responseError(resp)
	}
	return nil
}

func get(ctx context.Context, url, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create github API request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get github API response: %w", err)
	}
	return resp, nil
}

func responseError(resp *http.Response) error {
	body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxLoggedResponseBodyBytes+1))

	truncationMarker := ""
	if len(body) > maxLoggedResponseBodyBytes {
		body = body[:maxLoggedResponseBodyBytes]
		truncationMarker = fmt.Sprintf(" [response body truncated after %d bytes]", maxLoggedResponseBodyBytes)
	}

	finalURL := ""
	if resp.Request != nil && resp.Request.URL != nil {
		finalURL = resp.Request.URL.String()
	}
	return &APIError{
		StatusCode:         resp.StatusCode,
		URL:                finalURL,
		RequestID:          resp.Header.Get("X-GitHub-Request-Id"),
		RateLimitRemaining: resp.Header.Get("X-RateLimit-Remaining"),
		RateLimitReset:     resp.Header.Get("X-RateLimit-Reset"),
		RateLimitResource:  resp.Header.Get("X-RateLimit-Resource"),
		RetryAfter:         resp.Header.Get("Retry-After"),
		Body:               string(body) + truncationMarker,
		BodyReadError:      errorString(readErr),
	}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
