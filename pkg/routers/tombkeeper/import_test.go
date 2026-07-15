package tombkeeper

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimelineImporterStoresEntriesAndEmbeddedPostsWithDistinctMembership(t *testing.T) {
	entry := `{"id":"5314166504037012","bid":"ENTRY","user_id":"1401527553",` +
		`"screen_name":"tombkeeper","text":"timeline","pics":"",` +
		`"created_at":"$D2026-06-26T10:00:00.000Z","retweet_id":"5310000000000000","url_info":[]}`
	embedded := `{"id":"5310000000000000","bid":"ORIGINAL","user_id":"1401527553",` +
		`"screen_name":"tombkeeper","text":"embedded","pics":"",` +
		`"created_at":"$D2026-05-28T10:00:00.000Z","retweet_id":"","url_info":[]}`
	html := []byte(pushChunk("9:"+entry+"\n10:"+embedded+"\n") +
		`<a href="/weibo/5314166504037012"><span>详情</span></a>`)
	db := newFakeDB()
	importer := NewTimelineImporter(&fakeRequester{}, newFakeFile(), db, testLogger())

	stats, err := importer.Import(html)
	require.NoError(t, err)
	assert.Equal(t, ImportStats{EntriesSeen: 1, EntriesSaved: 1}, stats)
	assert.True(t, db.posts[5314166504037012].InTimeline)
	assert.False(t, db.posts[5310000000000000].InTimeline)
}

func TestTimelineImporterCountsEmbeddedPostWhenItBecomesTimelineEntry(t *testing.T) {
	db := newFakeDB()
	db.posts[5314166504037012] = &Post{ID: 5314166504037012, InTimeline: false}
	entry := `{"id":"5314166504037012","bid":"ENTRY","user_id":"1401527553",` +
		`"screen_name":"tombkeeper","text":"timeline","pics":"",` +
		`"created_at":"$D2026-06-26T10:00:00.000Z","retweet_id":"","url_info":[]}`
	html := []byte(pushChunk("9:"+entry+"\n") +
		`<a href="/weibo/5314166504037012"><span>详情</span></a>`)

	stats, err := NewTimelineImporter(&fakeRequester{}, newFakeFile(), db, testLogger()).Import(html)
	require.NoError(t, err)
	assert.Equal(t, ImportStats{EntriesSeen: 1, EntriesSaved: 1}, stats)
	assert.True(t, db.posts[5314166504037012].InTimeline)
}

func TestTimelineImporterDoesNotFetchDependenciesWhenStoredContentCannotBeLoaded(t *testing.T) {
	entry := `{"id":"5314166504037012","bid":"ENTRY","user_id":"1401527553",` +
		`"screen_name":"tombkeeper","text":"timeline","pics":"",` +
		`"created_at":"$D2026-06-26T10:00:00.000Z","retweet_id":"5310000000000000","url_info":[]}`
	html := []byte(pushChunk("9:"+entry+"\n") +
		`<a href="/weibo/5314166504037012"><span>详情</span></a>`)
	db := newFakeDB()
	db.getPostsErr = true
	req := &fakeRequester{details: map[string][]byte{
		"5310000000000000": detailPage("5310000000000000", "original", "text"),
	}}

	_, err := NewTimelineImporter(req, newFakeFile(), db, testLogger()).Import(html)
	require.ErrorContains(t, err, "load existing posts")
	assert.Zero(t, req.detailCalls)
}

func TestTimelineImporterUsesEmbeddedNonTombkeeperRetweetWithoutFetchingDetail(t *testing.T) {
	var fixture struct {
		Repost json.RawMessage `json:"repost"`
	}
	require.NoError(t, json.Unmarshal(readFixture(t, "view_pic_retweet.json"), &fixture))
	repost, err := parseSourcePost(fixture.Repost, nil)
	require.NoError(t, err)
	html := []byte(pushChunk("9:"+string(fixture.Repost)+"\n") +
		`<a href="/weibo/` + fmt.Sprint(repost.ID) + `"><span>详情</span></a>`)
	db := newFakeDB()
	req := &fakeRequester{picAvailable: true}

	_, err = NewTimelineImporter(req, newFakeFile(), db, testLogger()).Import(html)
	require.NoError(t, err)
	assert.Zero(t, req.detailCalls)
	original := db.posts[repost.RetweetPostID]
	require.NotNil(t, original)
	// autocorrect-disable（fixture 作者名，保持原样）
	assert.Equal(t, "数字热DGHOT", original.ScreenName)
	// autocorrect-enable
	assert.False(t, original.InTimeline)
}

func TestTimelineImporterUsesStoredRetweetWithoutFetchingDetail(t *testing.T) {
	const originalID = int64(5310000000000000)
	entry := `{"id":"5314166504037012","bid":"ENTRY","user_id":"1401527553",` +
		`"screen_name":"tombkeeper","text":"timeline","pics":"",` +
		`"created_at":"$D2026-06-26T10:00:00.000Z","retweet_id":"` + fmt.Sprint(originalID) + `","url_info":[]}`
	html := []byte(pushChunk("9:"+entry+"\n") +
		`<a href="/weibo/5314166504037012"><span>详情</span></a>`)
	db := newFakeDB()
	db.posts[originalID] = &Post{ID: originalID, ScreenName: "stored author", InTimeline: false}
	req := &fakeRequester{details: map[string][]byte{
		fmt.Sprint(originalID): detailPage(fmt.Sprint(originalID), "fetched author", "original body"),
	}}

	_, err := NewTimelineImporter(req, newFakeFile(), db, testLogger()).Import(html)
	require.NoError(t, err)
	assert.Zero(t, req.detailCalls)
	assert.Equal(t, "stored author", db.posts[originalID].ScreenName)
	assert.False(t, db.posts[originalID].InTimeline)
}

func TestTimelineImporterFetchesRetweetMissingFromPageAndStore(t *testing.T) {
	const originalID = "5310000000000000"
	entry := `{"id":"5314166504037012","bid":"ENTRY","user_id":"1401527553",` +
		`"screen_name":"tombkeeper","text":"timeline","pics":"",` +
		`"created_at":"$D2026-06-26T10:00:00.000Z","retweet_id":"` + originalID + `","url_info":[]}`
	html := []byte(pushChunk("9:"+entry+"\n") +
		`<a href="/weibo/5314166504037012"><span>详情</span></a>`)
	db := newFakeDB()
	req := &fakeRequester{details: map[string][]byte{
		originalID: detailPage(originalID, "external author", "original body"),
	}}

	_, err := NewTimelineImporter(req, newFakeFile(), db, testLogger()).Import(html)
	require.NoError(t, err)
	assert.Equal(t, 1, req.detailCalls)
	original := db.posts[5310000000000000]
	require.NotNil(t, original)
	assert.Equal(t, "external author", original.ScreenName)
	assert.False(t, original.InTimeline)
}

func TestTimelineImporterSummarizesFailedReferencedPostFetch(t *testing.T) {
	const originalID = "5310000000000000"
	entry := `{"id":"5314166504037012","bid":"ENTRY","user_id":"1401527553",` +
		`"screen_name":"tombkeeper","text":"timeline","pics":"",` +
		`"created_at":"$D2026-06-26T10:00:00.000Z","retweet_id":"` + originalID + `","url_info":[]}`
	html := []byte(pushChunk("9:"+entry+"\n") +
		`<a href="/weibo/5314166504037012"><span>详情</span></a>`)

	stats, err := NewTimelineImporter(&fakeRequester{}, newFakeFile(), newFakeDB(), testLogger()).Import(html)
	require.NoError(t, err)
	require.Equal(t, 1, stats.Failures.Count)
	require.Len(t, stats.Failures.Examples, 1)
	assert.Contains(t, stats.Failures.Examples[0], "fetch referenced post "+originalID)
}

func TestTimelineImporterSummarizesMissingTimelineEntries(t *testing.T) {
	html := []byte(`<a href="/weibo/5314166504037012"><span>详情</span></a>`)

	stats, err := NewTimelineImporter(&fakeRequester{}, newFakeFile(), newFakeDB(), testLogger()).Import(html)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.EntriesFailed)
	assert.Equal(t, 1, stats.Failures.Count)
	assert.Equal(t, []string{"timeline payload missing entries: 1"}, stats.Failures.Examples)
}

func TestTimelineImporterBoundsFailureExamples(t *testing.T) {
	var flight, links string
	for id := int64(5314166504037012); id < 5314166504037016; id++ {
		entry := fmt.Sprintf(`{"id":"%d","bid":"ENTRY","user_id":"1401527553",`+
			`"screen_name":"tombkeeper","text":"timeline","pics":"",`+
			`"created_at":"$D2026-06-26T10:00:00.000Z","retweet_id":"","url_info":[]}`, id)
		flight += fmt.Sprintf("%x:%s\n", id-5314166504037000, entry)
		links += fmt.Sprintf(`<a href="/weibo/%d"><span>详情</span></a>`, id)
	}
	db := newFakeDB()
	db.saveErr = true

	stats, err := NewTimelineImporter(&fakeRequester{}, newFakeFile(), db, testLogger()).Import(
		[]byte(pushChunk(flight) + links),
	)
	require.NoError(t, err)
	assert.Equal(t, 4, stats.Failures.Count)
	assert.Len(t, stats.Failures.Examples, maxFailureExamples)
}

func TestTimelineImporterStoresPostBeforeBestEffortImageAsset(t *testing.T) {
	source := loadSourcePost(t, "single_image.json")
	html := []byte(pushChunk("9:"+string(readFixture(t, "single_image.json"))+"\n") +
		`<a href="/weibo/` + fmt.Sprint(source.ID) + `"><span>详情</span></a>`)
	db := newFakeDB()
	db.imageSaveErr = true
	req := &fakeRequester{picAvailable: true}

	stats, err := NewTimelineImporter(req, newFakeFile(), db, testLogger()).Import(html)
	require.NoError(t, err)
	assert.Equal(t, 1, stats.EntriesSeen)
	assert.Equal(t, 1, stats.EntriesSaved)
	require.Equal(t, 1, stats.Failures.Count)
	require.Len(t, stats.Failures.Examples, 1)
	assert.Contains(t, stats.Failures.Examples[0], "archive image")
	assert.Contains(t, stats.Failures.Examples[0], fmt.Sprint(source.ID))
	assert.Contains(t, db.posts, source.ID)
}

func TestTimelineImporterReusesSuccessfulH5ImageIndex(t *testing.T) {
	html, repost, longURL := viewPicTimelinePage(t)
	req := &fakeRequester{
		picAvailable: true,
		reppics:      map[string][]string{longURL: {"53899d01ly1ief0r5kg95j210o2q6npd"}},
		reppicCalls:  map[string]int{},
	}
	db := newFakeDB()
	importer := NewTimelineImporter(req, newFakeFile(), db, testLogger())

	_, err := importer.Import(html)
	require.NoError(t, err)
	_, err = importer.Import(html)
	require.NoError(t, err)
	assert.Equal(t, 1, req.reppicCalls[longURL])
	got := db.posts[repost.ID].H5ImageIDsByURL[longURL]
	assert.Equal(t, []string{"53899d01ly1ief0r5kg95j210o2q6npd"}, got)
}

func TestTimelineImporterCachesSuccessfulEmptyH5ImageIndex(t *testing.T) {
	html, repost, longURL := viewPicTimelinePage(t)
	req := &fakeRequester{reppics: map[string][]string{longURL: {}}, reppicCalls: map[string]int{}}
	db := newFakeDB()
	importer := NewTimelineImporter(req, newFakeFile(), db, testLogger())

	_, err := importer.Import(html)
	require.NoError(t, err)
	_, err = importer.Import(html)
	require.NoError(t, err)
	assert.Equal(t, 1, req.reppicCalls[longURL])
	ids, exists := db.posts[repost.ID].H5ImageIDsByURL[longURL]
	assert.True(t, exists)
	assert.Empty(t, ids)
	assert.NotNil(t, ids)
	encoded, err := json.Marshal(ids)
	require.NoError(t, err)
	assert.JSONEq(t, "[]", string(encoded))
}

func TestTimelineImporterRetriesFailedH5Request(t *testing.T) {
	html, repost, longURL := viewPicTimelinePage(t)
	req := &fakeRequester{reppicErr: true, reppicCalls: map[string]int{}}
	db := newFakeDB()
	importer := NewTimelineImporter(req, newFakeFile(), db, testLogger())

	stats, err := importer.Import(html)
	require.NoError(t, err)
	require.Equal(t, 1, stats.Failures.Count)
	require.Len(t, stats.Failures.Examples, 1)
	assert.Contains(t, stats.Failures.Examples[0], "resolve H5 image index")
	assert.Contains(t, stats.Failures.Examples[0], fmt.Sprint(repost.ID))
	if _, exists := db.posts[repost.ID].H5ImageIDsByURL[longURL]; exists {
		t.Fatal("failed H5 request must not create a completed index entry")
	}
	req.reppicErr = false
	req.reppics = map[string][]string{longURL: {"recovered"}}
	_, err = importer.Import(html)
	require.NoError(t, err)
	assert.Equal(t, 2, req.reppicCalls[longURL])
	assert.Equal(t, []string{"recovered"}, db.posts[repost.ID].H5ImageIDsByURL[longURL])
}

func viewPicTimelinePage(t *testing.T) ([]byte, SourcePost, string) {
	t.Helper()
	var fixture struct {
		Repost   json.RawMessage `json:"repost"`
		Original json.RawMessage `json:"original"`
	}
	require.NoError(t, json.Unmarshal(readFixture(t, "view_pic_retweet.json"), &fixture))
	repost, err := parseSourcePost(fixture.Repost, nil)
	require.NoError(t, err)
	longURL := repost.Links[0].LongURL
	html := []byte(pushChunk("9:"+string(fixture.Repost)+"\n10:"+string(fixture.Original)+"\n") +
		`<a href="/weibo/` + fmt.Sprint(repost.ID) + `"><span>详情</span></a>`)
	return html, repost, longURL
}
