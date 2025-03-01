package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/controller/common"
	githubHandler "github.com/eli-yip/rss-zero/internal/controller/github"
	zhihuHandler "github.com/eli-yip/rss-zero/internal/controller/zhihu"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *model) generateFeed() tea.Msg {
	input := m.input.Value()
	var result string
	var err error

	switch m.feedType {
	case feedTypeZhihu:
		result, err = m.generateZhihuFeed(input)
	case feedTypeGitHub:
		result, err = m.generateGitHubFeed(input)
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

func (m *model) generateGitHubFeed(userRepo string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/api/v1/feed/github/%s", m.serverURL, userRepo))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result githubHandler.Resp

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	resultText := fmt.Sprintf(`Official Version Subscription:
  FreshRSS: %s

Pre-release Version Subscription:
  FreshRSS: %s`,
		result.Normal.FreshRSS,
		result.Pre.FreshRSS)

	return resultText, nil
}
