package macked

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type (
	WPTitle struct {
		Rendered string `json:"rendered"`
	}

	WPContent struct {
		Rendered string `json:"rendered"`
	}

	WPPost struct {
		ID       int       `json:"id"`
		Date     string    `json:"date_gmt"`
		Modified string    `json:"modified_gmt"`
		Slug     string    `json:"slug"`
		Link     string    `json:"link"`
		Title    WPTitle   `json:"title"`
		Content  WPContent `json:"content"`
	}
)

func GetLatestPosts() (posts []WPPost, err error) {
	const pageSize = 30
	posts = make([]WPPost, pageSize)

	resp, err := http.Get(fmt.Sprintf("https://macked.app/wp-json/wp/v2/posts?orderby=modified&order=desc&per_page=%d", pageSize))
	if err != nil {
		return nil, fmt.Errorf("failed to get latest posts: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get latest posts, bad status code: %d", resp.StatusCode)
	}

	if err = json.NewDecoder(resp.Body).Decode(&posts); err != nil {
		return nil, fmt.Errorf("failed to decode latest posts: %w", err)
	}

	return posts, nil
}
