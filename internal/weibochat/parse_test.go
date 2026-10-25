package weibochat

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseMessages(t *testing.T) {
	data, err := os.ReadFile("testdata/messages.json")
	require.NoError(t, err)
	var raw []json.RawMessage
	require.NoError(t, json.Unmarshal(data, &raw))
	messages := make([]Message, len(raw))
	for i, item := range raw {
		messages[i], err = ParseMessage(item)
		require.NoError(t, err)
		require.Empty(t, messages[i].ParseError)
	}
	require.Equal(t, MessageID("9007199254740993"), messages[0].MessageID)
	require.Equal(t, "你好\n[doge] 🐱", messages[0].Text)
	require.NotContains(t, string(messages[0].Raw), "attitude_info")
	require.EqualValues(t, 1792907148, messages[0].SentAt.Unix())
	require.Equal(t, []AssetRef{{Kind: "image", AssetKey: "file/765"}}, messages[1].Content.Assets)
	require.Equal(t, []AssetRef{{Kind: "video", AssetKey: "file/766"}}, messages[2].Content.Assets)
	require.Equal(t, "pic/sticker1", messages[3].Content.Assets[0].AssetKey)
	require.Equal(t, "https://weibo.com/example", messages[4].Content.Links[0].URL)
	require.Equal(t, "pic/card1", messages[4].Content.Assets[0].AssetKey)
	require.Contains(t, string(messages[5].Raw), "unknown")
	require.Empty(t, messages[6].Content.Assets)
}

func TestInvalidIdentityAndRecoverableContent(t *testing.T) {
	for _, raw := range []string{`null`, `[]`, `{"id":1}`, `{"gid":1,"id":1.5}`, `{"gid":1,"id":"01"}`} {
		_, err := ParseMessage([]byte(raw))
		require.Error(t, err)
	}
	message, err := ParseMessage([]byte(`{"gid":1,"id":2,"content":{"unexpected":true}}`))
	require.NoError(t, err)
	require.NotEmpty(t, message.ParseError)
	require.Contains(t, string(message.Raw), "unexpected")
}

func TestMessageIDComparison(t *testing.T) {
	require.Positive(t, MessageID("100").Compare("99"))
	require.Positive(t, MessageID("9007199254740993").Compare("9007199254740992"))
	require.Zero(t, MessageID("1").Compare("1"))
}
