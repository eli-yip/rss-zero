package request

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

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

func GetRepoReleases(user, repo, token string) (releases []Release, err error) {
	releases = make([]Release, 0)

	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", user, repo), nil)
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
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get releases API response, bad status code: %d", resp.StatusCode)
	}

	if err = json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode releases API response: %w", err)
	}

	if len(releases) == 0 {
		return nil, fmt.Errorf("releases API response is empty")
	}

	return releases, nil
}
