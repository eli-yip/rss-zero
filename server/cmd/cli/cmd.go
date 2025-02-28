package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/controller/common"
	zhihuHandler "github.com/eli-yip/rss-zero/internal/controller/zhihu"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *model) generateFeed() tea.Msg {
	input := m.input.Value()
	var result string
	var err error

	switch m.feedType {
	case "Zhihu Subscribe":
		result, err = m.generateZhihuFeed(input)
		// case "RSSHub Subscribe":
		// 	result, err = m.generateRSSHubFeed(input)
		// case "GitHub Release Subscribe":
		// 	result, err = m.generateGitHubFeed(input)
	}

	return feedResultMsg{
		result: result,
		err:    err,
	}
}

func (m *model) generateZhihuFeed(authorID string) (string, error) {
	// Create request with Basic Auth
	req, err := http.NewRequest(http.MethodGet,
		fmt.Sprintf("%s/api/v1/feed/zhihu/%s", m.serverURL, authorID),
		nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(config.C.Settings.Username, config.C.Settings.Password)

	// Send request
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status code: %d", resp.StatusCode)
	}

	// Define response structure using the same types from feed.go
	var result common.ApiResp[zhihuHandler.FeedResp]

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	// Format the output
	resultText := fmt.Sprintf(`Answer Subscription: %s
Article Subscription: %s
Idea Subscription: %s`,
		result.Data.FreshRSS.AnswerFeed,
		result.Data.FreshRSS.ArticleFeed,
		result.Data.FreshRSS.PinFeed)

	return resultText, nil
}

// func (m *model) generateRSSHubFeed(username string) (string, error) {
// 	payload := map[string]string{
// 		"feed_type": "weibo",
// 		"username":  username,
// 	}
// 	jsonData, err := json.Marshal(payload)
// 	if err != nil {
// 		return "", err
// 	}

// 	resp, err := http.Post(fmt.Sprintf("%s/api/v1/feed/rsshub", m.serverURL),
// 		"application/json", bytes.NewBuffer(jsonData))
// 	if err != nil {
// 		return "", err
// 	}
// 	defer resp.Body.Close()

// 	var result struct {
// 		Data struct {
// 			FeedURL string `json:"feed_url"`
// 		} `json:"data"`
// 	}

// 	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
// 		return "", err
// 	}

// 	return fmt.Sprintf("Subscription URL:\n%s", result.Data.FeedURL), nil
// }

// func (m *model) generateGitHubFeed(userRepo string) (string, error) {
// 	resp, err := http.Get(fmt.Sprintf("%s/api/v1/feed/github/%s", m.serverURL, userRepo))
// 	if err != nil {
// 		return "", err
// 	}
// 	defer resp.Body.Close()

// 	var result struct {
// 		Normal struct {
// 			External string `json:"external"`
// 			Internal string `json:"internal"`
// 			FreshRSS string `json:"fresh_rss"`
// 		} `json:"normal"`
// 		Pre struct {
// 			External string `json:"external"`
// 			Internal string `json:"internal"`
// 			FreshRSS string `json:"fresh_rss"`
// 		} `json:"pre"`
// 	}

// 	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
// 		return "", err
// 	}

// 	resultText := fmt.Sprintf(`Official Version Subscription:
//   External Access: %s
//   Internal Access: %s
//   FreshRSS: %s

// Pre-release Version Subscription:
//   External Access: %s
//   Internal Access: %s
//   FreshRSS: %s`,
// 		result.Normal.External, result.Normal.Internal, result.Normal.FreshRSS,
// 		result.Pre.External, result.Pre.Internal, result.Pre.FreshRSS)

// 	return resultText, nil
// }
