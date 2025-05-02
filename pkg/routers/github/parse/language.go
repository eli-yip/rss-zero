package parse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/eli-yip/rss-zero/config"
	"github.com/rs/xid"
)

type Language int

const (
	LanguageUnknown Language = iota
	LanguageChinese
	LanguageEnglish
)

func (s *ParseService) detectLanguage(text string) (language Language, exists bool, err error) {
	raw := map[string]string{
		"request_id": xid.New().String(),
		"content":    text,
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return LanguageUnknown, false, fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/detect", config.C.LanguageDetection.Server), bytes.NewBuffer(body))
	if err != nil {
		return LanguageUnknown, false, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return LanguageUnknown, false, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return LanguageUnknown, false, fmt.Errorf("failed to detect language: %s", resp.Status)
	}

	var result struct {
		Language string `json:"language"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return LanguageUnknown, false, fmt.Errorf("failed to decode response: %w", err)
	}

	switch result.Language {
	case "english":
		return LanguageEnglish, true, nil
	case "chinese":
		return LanguageChinese, true, nil
	default:
		return LanguageUnknown, false, nil
	}
}
