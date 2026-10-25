package weibochat

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/stretchr/testify/require"
)

func TestMessageContextAndBidirectionalContinuation(t *testing.T) {
	e, db := testController(t)
	seedMessages(t, db)
	var result httputil.Resp[MessageContextResponse]
	rec := get(t, e, "/groups/1/messages/100/context?before_limit=1&after_limit=1&highlight_q=image", http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Equal(t, "100", result.Data.AnchorMessageID)
	require.Equal(t, []string{"95", "100", "101"}, responseIDs(result.Data.Items))
	require.False(t, result.Data.HasMoreBefore)
	require.True(t, result.Data.HasMoreAfter)
	require.Equal(t, []weibochat.TextMatch{{Start: 0, End: 5}}, result.Data.Items[0].TextMatches)
	require.Empty(t, result.Data.Items[1].TextMatches)
	require.Equal(t, "latest name", result.Data.Items[0].Sender.ScreenName)
	require.Len(t, result.Data.Items[0].Assets, 3)
	require.NotContains(t, rec.Body.String(), "private")
	// Continue the full context without search filters; highlight is independent.
	var page httputil.Resp[MessageListResponse]
	rec = get(t, e, "/groups/1/messages?after_message_id=101&highlight_q=image&limit=1", http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	require.Equal(t, []string{"102"}, responseIDs(page.Data.Items))
	require.False(t, page.Data.HasMore)
	require.Empty(t, page.Data.Items[0].TextMatches)
	rec = get(t, e, "/groups/1/messages?after_message_id=95&q=unknown&highlight_q=KNOWN&limit=1", http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page))
	require.Equal(t, []string{"101"}, responseIDs(page.Data.Items))
	require.Equal(t, []weibochat.TextMatch{{Start: 2, End: 7}}, page.Data.Items[0].TextMatches)
	require.False(t, page.Data.HasMore)

	rec = get(t, e, "/groups/1/messages/100/context?before_limit=0&after_limit=0", http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Equal(t, []string{"100"}, responseIDs(result.Data.Items))
	require.True(t, result.Data.HasMoreBefore)
	require.True(t, result.Data.HasMoreAfter)
	for _, path := range []string{"/groups/1/messages/900/context", "/groups/2/messages/100/context", "/groups/1/messages/99/context"} {
		get(t, e, path, http.StatusNotFound)
	}
}

func TestMessageContextParameters(t *testing.T) {
	e := testRoutes(NewController(nil, nil))
	for _, query := range []string{
		"before_limit=", "before_limit=-1", "before_limit=51", "before_limit=1&before_limit=2", "after_limit=51", "after_limit=bad",
		"q=hello", "sender_id=1", "sent_from=2026-10-25T00:00:00Z", "sent_before=2026-10-26T00:00:00Z", "asset_kind=image",
		"before_message_id=1", "after_message_id=1", "limit=1", "highlight_q=%22", "highlight_q=a&highlight_q=b",
	} {
		get(t, e, "/groups/1/messages/100/context?"+query, http.StatusBadRequest)
	}
	get(t, e, "/groups/invalid/messages/100/context", http.StatusBadRequest)
	get(t, e, "/groups/1/messages/invalid/context", http.StatusBadRequest)
}

func responseIDs(messages []MessageResponse) []string {
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.MessageID)
	}
	return ids
}
