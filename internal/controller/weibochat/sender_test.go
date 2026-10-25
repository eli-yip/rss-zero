package weibochat

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/stretchr/testify/require"
)

func TestSendersUseGroupMessagesAndLatestProfiles(t *testing.T) {
	e, db := testController(t)
	seedMessages(t, db)
	var result httputil.Resp[SenderListResponse]
	rec := get(t, e, "/groups/1/senders", http.StatusOK)
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
	require.Len(t, result.Data.Items, 3)
	require.Equal(t, "10", result.Data.Items[0].UID)
	require.Equal(t, "latest name", result.Data.Items[0].ScreenName)
	require.Equal(t, "https://assets.example/avatars/a%20b.jpg", *result.Data.Items[0].AvatarURL)
	require.Equal(t, "30", result.Data.Items[2].UID)
	require.Empty(t, result.Data.Items[2].ScreenName)
	require.Nil(t, result.Data.Items[2].AvatarURL)
	rec = get(t, e, "/groups/999/senders", http.StatusOK)
	require.JSONEq(t, `{"message":"weibo senders","data":{"items":[]}}`, rec.Body.String())
}
