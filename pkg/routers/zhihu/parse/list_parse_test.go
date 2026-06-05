package parse

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestParseAnswerListInvalidPayload(t *testing.T) {
	parser := &ParseService{}
	logger := zap.NewNop()

	testCases := []struct {
		name    string
		payload []byte
		wantErr string
	}{
		{
			name:    "empty payload",
			payload: []byte(""),
			wantErr: "failed to unmarshal answer list: unexpected end of JSON input",
		},
		{
			name:    "truncated json",
			payload: []byte(`{"data":[{"id":1001`),
			wantErr: "failed to unmarshal answer list: unexpected end of JSON input",
		},
		{
			name:    "html payload",
			payload: []byte(`<html>bad gateway</html>`),
			wantErr: "failed to unmarshal answer list: invalid character '<' looking for beginning of value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := parser.ParseAnswerList(tc.payload, 0, logger)

			require.Error(t, err)
			assert.EqualError(t, err, tc.wantErr)
		})
	}
}

func TestParseAnswerListSuccess(t *testing.T) {
	parser := &ParseService{}

	payload := []byte(`{
		"paging":{"is_end":true,"totals":1,"next":"https://example.com/next"},
		"data":[{
			"id":"1001",
			"created_time":1710000000,
			"updated_time":1710000100,
			"question":{"id":2001,"created":1700000000,"title":"Question title"},
			"author":{"url_token":"author","name":"Author"},
			"content":"<p>answer</p>"
		}]
	}`)

	paging, excerpts, rawAnswers, err := parser.ParseAnswerList(payload, 3, zap.NewNop())

	require.NoError(t, err)
	assert.True(t, paging.IsEnd)
	assert.Equal(t, 1, paging.Totals)
	assert.Equal(t, "https://example.com/next", paging.Next)
	require.Len(t, excerpts, 1)
	assert.Equal(t, 1001, excerpts[0].ID)
	assert.Equal(t, 2001, excerpts[0].Question.ID)
	assert.Equal(t, "Question title", excerpts[0].Question.Title)
	require.Len(t, rawAnswers, 1)
	assert.Contains(t, string(rawAnswers[0]), `"content":"<p>answer</p>"`)
}

func TestParseArticleListSuccess(t *testing.T) {
	parser := &ParseService{}

	payload := []byte(`{
		"paging":{"is_end":false,"totals":2,"next":"https://example.com/articles?offset=20"},
		"data":[{
			"id":3001,
			"created":1710000000,
			"updated":1710000100,
			"author":{"url_token":"author","name":"Author"},
			"title":"Article title",
			"content":"<p>article</p>"
		}]
	}`)

	paging, excerpts, rawArticles, err := parser.ParseArticleList(payload, 1, zap.NewNop())

	require.NoError(t, err)
	assert.False(t, paging.IsEnd)
	assert.Equal(t, 2, paging.Totals)
	require.Len(t, excerpts, 1)
	assert.Equal(t, 3001, excerpts[0].ID)
	assert.Equal(t, "Article title", excerpts[0].Title)
	require.Len(t, rawArticles, 1)
	assert.Contains(t, string(rawArticles[0]), `"title":"Article title"`)
}

func TestParsePinListSuccess(t *testing.T) {
	parser := &ParseService{}

	payload := []byte(`{
		"paging":{"is_end":true,"totals":1},
		"data":[{
			"id":"4001",
			"created":1710000000,
			"updated":1710000100,
			"author":{"url_token":"author","name":"Author"},
			"content":[{"type":"text","content":"<p>pin</p>"}]
		}]
	}`)

	paging, excerpts, rawPins, err := parser.ParsePinList(payload, 4, zap.NewNop())

	require.NoError(t, err)
	assert.True(t, paging.IsEnd)
	assert.Equal(t, 1, paging.Totals)
	require.Len(t, excerpts, 1)
	assert.Equal(t, "4001", excerpts[0].ID)
	assert.Equal(t, int64(1710000000), excerpts[0].CreateAt)
	require.Len(t, rawPins, 1)
	assert.Contains(t, string(rawPins[0]), `"id":"4001"`)
}

func TestListPayloadPreviewLimitStaysSmall(t *testing.T) {
	payload := []byte(strings.Repeat("x", listPayloadPreviewLimit*2))

	assert.Len(t, previewPayload(payload, listPayloadPreviewLimit), listPayloadPreviewLimit)
	assert.Len(t, tailPayload(payload, listPayloadPreviewLimit), listPayloadPreviewLimit)
}
