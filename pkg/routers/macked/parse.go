package macked

import (
	"fmt"
	"html"
	"strconv"
	"time"
)

type (
	ParsedPost struct {
		ID        string    `json:"id"`
		Published time.Time `json:"published"`
		Modified  time.Time `json:"modified"`
		Title     string    `json:"title"`
		Content   string    `json:"content"`
		Link      string    `json:"link"`
	}
)

func ParsePosts(posts []WPPost) (parsedPosts []ParsedPost, err error) {
	parsedPosts = make([]ParsedPost, 0, len(posts))

	for _, p := range posts {
		idStr := strconv.Itoa(p.ID)
		published, err := parseTime(p.Date)
		if err != nil {
			return nil, fmt.Errorf("failed to parse published time: %w", err)
		}
		modified, err := parseTime(p.Modified)
		if err != nil {
			return nil, fmt.Errorf("failed to parse modified time: %w", err)
		}

		if modified.IsZero() {
			modified = published
		}

		parsedPosts = append(parsedPosts, ParsedPost{
			ID:        idStr,
			Published: published,
			Modified:  modified,
			Title:     html.UnescapeString(p.Title.Rendered),
			Content:   html.UnescapeString(p.Content.Rendered),
			Link:      p.Link,
		})
	}

	return parsedPosts, nil
}

// 2024-08-07T05:52:25
func parseTime(t string) (time.Time, error) {
	return time.Parse("2006-01-02T15:04:05", t)
}
