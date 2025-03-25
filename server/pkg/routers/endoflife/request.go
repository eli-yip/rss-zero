package endoflife

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// One cycle of https://endoflife.date/api/{product}.json response
type cycle struct {
	Cycle             string `json:"cycle"`
	ReleaseDate       string `json:"releaseDate"`
	Eol               any    `json:"eol"`
	Latest            string `json:"latest"`
	LatestReleaseDate string `json:"latestReleaseDate"`
	Lts               bool   `json:"lts"`
}

func GetReleaseCycles(product string) (cycles []cycle, err error) {
	cycles = make([]cycle, 0)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, fmt.Sprintf("https://endoflife.date/api/%s.json", product), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest release API response: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get latest release API response, bad status code: %d", resp.StatusCode)
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read latest release API response into bytes: %w", err)
	}

	if err = json.Unmarshal(bytes, &cycles); err != nil {
		return nil, fmt.Errorf("failed to unmarshal latest release API response: %w", err)
	}

	if len(cycles) == 0 {
		return nil, fmt.Errorf("latest release API response is empty")
	}

	return cycles, nil
}
