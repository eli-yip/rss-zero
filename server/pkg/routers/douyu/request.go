package douyu

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"slices"
)

type RequestOption func(*http.Request)

func WithReferer(r string) RequestOption {
	return func(req *http.Request) {
		req.Header.Set("Referer", r)
	}
}

func requestUrl(ctx context.Context, u string, opts ...RequestOption) (data []byte, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to new request: %v", err)
	}

	for opt := range slices.Values(opts) {
		opt(req)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to do request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to do request, bad status code: %d", resp.StatusCode)
	}

	data, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %v", err)
	}

	return data, nil
}
