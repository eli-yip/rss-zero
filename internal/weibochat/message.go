// Package weibochat archives Weibo group messages independently of RSS.
package weibochat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// MessageID preserves decimal IDs without floating-point conversion.
type MessageID string

func (id MessageID) Valid() bool {
	if id == "" || id[0] == '0' {
		return false
	}
	for _, c := range id {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// Compare compares validated IDs numerically. Empty IDs sort before valid IDs.
func (id MessageID) Compare(other MessageID) int {
	if len(id) < len(other) {
		return -1
	}
	if len(id) > len(other) {
		return 1
	}
	return strings.Compare(string(id), string(other))
}

func (id *MessageID) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	var value string
	if len(data) > 0 && data[0] == '"' {
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
	} else {
		value = string(data)
	}
	parsed := MessageID(value)
	if !parsed.Valid() {
		return fmt.Errorf("invalid decimal ID")
	}
	*id = parsed
	return nil
}

// Message retains the first observation. Raw excludes reaction metadata.
type Message struct {
	GroupID     string
	MessageID   MessageID
	SenderID    string
	SentAt      *time.Time
	MessageType *int
	MediaType   *int
	SubType     *int
	Text        string
	Raw         json.RawMessage
	Content     MessageContent
	ParseError  string
	FetchedAt   time.Time
}

type MessageContent struct {
	SenderName string     `json:"sender_name,omitempty"`
	AvatarURL  string     `json:"avatar_url,omitempty"`
	Links      []Link     `json:"links,omitempty"`
	Assets     []AssetRef `json:"assets,omitempty"`
}

type Link struct {
	OriginalURL string `json:"original_url,omitempty"`
	ShortURL    string `json:"short_url,omitempty"`
	URL         string `json:"url,omitempty"`
}

// AssetRef identifies a resource within the message's group. Slice order is display order.
type AssetRef struct {
	Kind      string `json:"kind"`
	AssetKey  string `json:"asset_key"`
	SourceURL string `json:"source_url,omitempty"`
}
