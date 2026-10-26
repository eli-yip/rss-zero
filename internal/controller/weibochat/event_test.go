package weibochat

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/stretchr/testify/require"
)

func TestEventResponsesDoNotInheritSenderPreferences(t *testing.T) {
	e, db := testController(t)
	require.NoError(t, db.Create(&weibochat.User{UID: "42", ScreenName: "current profile", ProfileObservedAt: time.Now().UTC()}).Error)
	require.NoError(t, db.Create(&weibochat.UserPreference{OwnerUsername: "test-user", UID: "42", IsStarred: true, IsBlocked: true}).Error)
	messages := []weibochat.Message{
		{GroupID: "1", MessageID: "1", SenderID: "42", Raw: []byte(`{}`), FetchedAt: time.Now().UTC()},
		{GroupID: "1", MessageID: "2", SenderID: "42", EventType: weibochat.EventWeiboPublished, Text: "我发布了新微博，快来围观吧！", Raw: []byte(`{}`), FetchedAt: time.Now().UTC(), Content: weibochat.MessageContent{EventActor: &weibochat.EventActor{UID: "42", ScreenName: "original publisher"}, Links: []weibochat.Link{{URL: "https://weibo.com/42/abc"}}}},
	}
	require.NoError(t, db.Create(&messages).Error)
	for _, path := range []string{"/groups/1/messages", "/groups/1/messages/2/context?before_limit=1&after_limit=0"} {
		rec := get(t, e, path, http.StatusOK)
		var envelope struct {
			Data struct {
				Items []map[string]any `json:"items"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
		require.Len(t, envelope.Data.Items, 2)
		ordinary, event := envelope.Data.Items[0], envelope.Data.Items[1]
		require.Equal(t, "", ordinary["event_type"])
		require.Nil(t, ordinary["event_actor"])
		require.Equal(t, "weibo_published", event["event_type"])
		require.Equal(t, "", event["sender_id"])
		require.Nil(t, event["sender"])
		require.Equal(t, false, event["is_sender_starred"])
		require.Equal(t, false, event["is_sender_blocked"])
		require.Equal(t, map[string]any{"uid": "42", "screen_name": "original publisher"}, event["event_actor"])
		require.Equal(t, []any{map[string]any{"url": "https://weibo.com/42/abc"}}, event["links"])
	}
}
