package weibochat

import (
	"time"

	"github.com/eli-yip/rss-zero/internal/file"
	"github.com/eli-yip/rss-zero/internal/weibochat"
)

type GroupResponse struct {
	GroupID       string     `json:"group_id"`
	Configured    bool       `json:"configured"`
	InitializedAt *time.Time `json:"initialized_at"`
	LastSyncedAt  *time.Time `json:"last_synced_at"`
}

type GroupListResponse struct {
	Items []GroupResponse `json:"items"`
}

type SenderResponse struct {
	UID        string  `json:"uid"`
	ScreenName string  `json:"screen_name"`
	AvatarURL  *string `json:"avatar_url"`
}

type SenderListResponse struct {
	Items []SenderResponse `json:"items"`
}

type AssetResponse struct {
	AssetKey    string  `json:"asset_key"`
	Kind        string  `json:"kind"`
	Status      string  `json:"status"`
	URL         *string `json:"url"`
	ContentType string  `json:"content_type"`
	ByteSize    int64   `json:"byte_size"`
}

type MessageResponse struct {
	TextMatches []weibochat.TextMatch `json:"text_matches"`
	GroupID     string                `json:"group_id"`
	MessageID   string                `json:"message_id"`
	SenderID    string                `json:"sender_id"`
	Sender      *SenderResponse       `json:"sender"`
	SentAt      *time.Time            `json:"sent_at"`
	Text        string                `json:"text"`
	Links       []weibochat.Link      `json:"links"`
	Assets      []AssetResponse       `json:"assets"`
	MessageType *int                  `json:"message_type"`
	MediaType   *int                  `json:"media_type"`
	SubType     *int                  `json:"sub_type"`
	ParseStatus string                `json:"parse_status"`
}

type MessageListResponse struct {
	Items               []MessageResponse `json:"items"`
	NextBeforeMessageID *string           `json:"next_before_message_id"`
}

func senderResponse(user weibochat.User) SenderResponse {
	return SenderResponse{UID: user.UID, ScreenName: user.ScreenName, AvatarURL: storedURL(user.AvatarStorageProvider, user.AvatarObjectKey)}
}

func storedURL(provider, key string) *string {
	if provider == "" || key == "" {
		return nil
	}
	value, err := file.ObjectURI([]string{provider}, key)
	if err != nil {
		return nil
	}
	return &value
}

func utc(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	result := value.UTC()
	return &result
}
