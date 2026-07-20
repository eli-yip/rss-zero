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

const maxLoggedResponseBodyBytes = 64 * 1024

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

func GetRepoReleases(user, repo, token string) (releases []Release, err error) {
	releases = make([]Release, 0)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", user, repo), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create releases API request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get releases API response: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, maxLoggedResponseBodyBytes+1))
		if readErr != nil {
			return nil, fmt.Errorf("failed to get releases API response, bad status code: %d, failed to read response body: %w", resp.StatusCode, readErr)
		}

		truncationMarker := ""
		if len(body) > maxLoggedResponseBodyBytes {
			body = body[:maxLoggedResponseBodyBytes]
			truncationMarker = fmt.Sprintf(" [response body truncated after %d bytes]", maxLoggedResponseBodyBytes)
		}

		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("%w: status %d, response body: %s%s", ErrUnauthorized, resp.StatusCode, body, truncationMarker)
		}
		return nil, fmt.Errorf("failed to get releases API response, bad status code: %d, response body: %s%s", resp.StatusCode, body, truncationMarker)
	}

	if err = json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode releases API response: %w", err)
	}

	if len(releases) == 0 {
		return nil, ErrNoRelease
	}

	return releases, nil
}
