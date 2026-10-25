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
	GroupID     string     `gorm:"type:text;primaryKey;index:idx_weibo_chat_message_order,priority:1;index:idx_weibo_chat_message_sender,priority:1"`
	MessageID   MessageID  `gorm:"type:text;primaryKey;index:idx_weibo_chat_message_order,priority:2,expression:length(message_id),sort:desc;index:idx_weibo_chat_message_order,priority:3,collate:\"C\",sort:desc"`
	SenderID    string     `gorm:"type:text;index;index:idx_weibo_chat_message_sender,priority:2"`
	Sender      *User      `gorm:"-"`
	SentAt      *time.Time `gorm:"type:timestamptz;index"`
	MessageType *int
	MediaType   *int
	SubType     *int
	Text        string          `gorm:"type:text"`
	Raw         json.RawMessage `gorm:"type:jsonb;serializer:json;not null"`
	Content     MessageContent  `gorm:"type:jsonb;serializer:json;not null"`
	ParseError  string          `gorm:"type:text"`
	FetchedAt   time.Time       `gorm:"type:timestamptz;not null"`
}

func (Message) TableName() string { return "weibo_chat_message" }

type MessageContent struct {
	Links  []Link     `json:"links,omitempty"`
	Assets []AssetRef `json:"assets,omitempty"`
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

// MessageFilter selects archived messages within one group. Filter dimensions
// are combined with AND; multiple senders or asset kinds within a dimension use OR.
type MessageFilter struct {
	Search          SearchQuery
	SenderIDs       []string
	SentFrom        *time.Time
	SentBefore      *time.Time
	AssetKinds      []string
	BeforeMessageID MessageID
	AfterMessageID  MessageID
	Limit           int
}

// MessageListPage always presents messages in ascending numeric ID order.
// HasMore refers to the requested direction, or older messages without a cursor.
type MessageListPage struct {
	Messages []Message
	HasMore  bool
}

// MessageContext contains an anchor and its nearest archived neighbors.
type MessageContext struct {
	AnchorMessageID MessageID
	Messages        []Message
	HasMoreBefore   bool
	HasMoreAfter    bool
}
