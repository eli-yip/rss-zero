package tombkeeper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractTimelinePageClassifiesEntriesAndEmbeddedPosts(t *testing.T) {
	entry := `{"id":"5314166504037012","bid":"ENTRY","user_id":"1401527553",` +
		`"screen_name":"tombkeeper","text":"timeline","pics":"",` +
		`"created_at":"$D2026-06-26T10:00:00.000Z","retweet_id":"5310000000000000","url_info":[]}`
	embedded := `{"id":"5310000000000000","bid":"ORIGINAL","user_id":"1401527553",` +
		`"screen_name":"tombkeeper","text":"embedded","pics":"",` +
		`"created_at":"$D2026-05-28T10:00:00.000Z","retweet_id":"","url_info":[]}`
	html := []byte(pushChunk("9:"+entry+"\n10:"+embedded+"\n") +
		`<a href="/weibo/5314166504037012"><span>详情</span></a>`)

	page, err := ExtractTimelinePage(html)
	require.NoError(t, err)
	require.Len(t, page.Entries, 1)
	assert.EqualValues(t, 5314166504037012, page.Entries[0].ID)
	require.Len(t, page.EmbeddedPosts, 1)
	assert.EqualValues(t, 5310000000000000, page.EmbeddedPosts[0].ID)
	assert.Zero(t, page.MissingEntries)
}

func TestExtractTimelinePageCountsMissingEntry(t *testing.T) {
	embedded := `{"id":"5310000000000000","bid":"ORIGINAL","user_id":"1401527553",` +
		`"screen_name":"tombkeeper","text":"embedded","pics":"",` +
		`"created_at":"$D2026-05-28T10:00:00.000Z","retweet_id":"","url_info":[]}`
	html := []byte(pushChunk("9:"+embedded+"\n") +
		`<a href="/weibo/5314166504037012"><span>详情</span></a>`)

	page, err := ExtractTimelinePage(html)
	require.NoError(t, err)
	assert.Equal(t, 1, page.MissingEntries)
	assert.Empty(t, page.Entries)
	assert.Len(t, page.EmbeddedPosts, 1)
}

func TestExtractTimelinePageCountsEntryWithInvalidPublishedAt(t *testing.T) {
	invalid := `{"id":"5314166504037012","bid":"ENTRY","user_id":"1401527553",` +
		`"screen_name":"tombkeeper","text":"timeline","pics":"",` +
		`"created_at":"not-a-time","retweet_id":"","url_info":[]}`
	html := []byte(pushChunk("9:"+invalid+"\n") +
		`<a href="/weibo/5314166504037012"><span>详情</span></a>`)

	page, err := ExtractTimelinePage(html)
	require.NoError(t, err)
	assert.Equal(t, 1, page.MissingEntries)
	assert.Empty(t, page.Entries)
}
