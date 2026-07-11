package tombkeeper

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/eli-yip/rss-zero/internal/md"
)

func importAndRenderObject(t *testing.T, raw []byte, req *fakeRequester) (Post, string, *fakeDB, *fakeFile) {
	t.Helper()
	source, err := parseSourcePost(raw, nil)
	if err != nil {
		t.Fatal(err)
	}
	html := []byte(pushChunk("9:"+string(raw)+"\n") +
		fmt.Sprintf(`<a href="/weibo/%d"><span>详情</span></a>`, source.ID))
	db := newFakeDB()
	files := newFakeFile()
	if _, err := NewTimelineImporter(req, files, db, testLogger()).Import(html); err != nil {
		t.Fatal(err)
	}
	post := *db.posts[source.ID]
	content, err := NewContentLoader(db).Load([]Post{post})
	if err != nil {
		t.Fatal(err)
	}
	markdown, err := RenderMarkdown(post.ID, content, "https://srv.test")
	if err != nil {
		t.Fatal(err)
	}
	return post, markdown, db, files
}

func importAndRenderPair(t *testing.T, name string, req *fakeRequester) (SourcePost, SourcePost, string, *fakeDB, *fakeFile) {
	t.Helper()
	var fixture struct {
		Repost   json.RawMessage `json:"repost"`
		Original json.RawMessage `json:"original"`
	}
	if err := json.Unmarshal(readFixture(t, name), &fixture); err != nil {
		t.Fatal(err)
	}
	repost, err := parseSourcePost(fixture.Repost, nil)
	if err != nil {
		t.Fatal(err)
	}
	original, err := parseSourcePost(fixture.Original, nil)
	if err != nil {
		t.Fatal(err)
	}
	html := []byte(pushChunk("9:"+string(fixture.Repost)+"\n10:"+string(fixture.Original)+"\n") +
		fmt.Sprintf(`<a href="/weibo/%d"><span>详情</span></a>`, repost.ID))
	db := newFakeDB()
	files := newFakeFile()
	if _, err := NewTimelineImporter(req, files, db, testLogger()).Import(html); err != nil {
		t.Fatal(err)
	}
	root := *db.posts[repost.ID]
	content, err := NewContentLoader(db).Load([]Post{root})
	if err != nil {
		t.Fatal(err)
	}
	markdown, err := RenderMarkdown(root.ID, content, "https://srv.test")
	if err != nil {
		t.Fatal(err)
	}
	return repost, original, markdown, db, files
}

func TestImportAndRenderSingleImage(t *testing.T) {
	post, markdown, db, files := importAndRenderObject(t, readFixture(t, "single_image.json"),
		&fakeRequester{picAvailable: true})
	picID := picIDOf(post.Pics[0])
	wantURL := "https://oss.test/rss/tombkeeper/" + picID + ".jpg"
	if !strings.Contains(markdown, "![微博图片 1]("+wantURL+")") {
		t.Errorf("markdown missing image %s:\n%s", wantURL, markdown)
	}
	if asset, _ := db.GetImageAsset(picID); asset == nil || asset.Status != ObjectStatusOK {
		t.Errorf("image asset not saved: %+v", asset)
	}
	if _, ok := files.saved["tombkeeper/"+picID+".jpg"]; !ok {
		t.Fatal("image not uploaded")
	}
}

func TestImportAndRenderMultiImageOrdered(t *testing.T) {
	post, markdown, _, _ := importAndRenderObject(t, readFixture(t, "multi_image.json"),
		&fakeRequester{picAvailable: true})
	last := -1
	for _, rawImage := range post.Pics {
		url := "https://oss.test/rss/tombkeeper/" + picIDOf(rawImage) + ".jpg"
		index := strings.Index(markdown, url)
		if index < last {
			t.Fatalf("images out of order:\n%s", markdown)
		}
		last = index
	}
}

func TestImportAndRenderAbandonedImageShowsSourceNotice(t *testing.T) {
	post, markdown, db, _ := importAndRenderObject(t, readFixture(t, "single_image.json"),
		&fakeRequester{picAvailable: false})
	picID := picIDOf(post.Pics[0])
	want := md.Quote("微博图片 1 下载失败，请前往 [源微博](" +
		WeiboPostURL(post.AuthorID, post.Bid, strconv.FormatInt(post.ID, 10)) + ") 查看")
	if !strings.Contains(markdown, want) {
		t.Fatalf("missing failure notice %q:\n%s", want, markdown)
	}
	if asset, _ := db.GetImageAsset(picID); asset == nil || asset.Status != ObjectStatusAbandoned {
		t.Fatalf("image should be abandoned: %+v", asset)
	}
}

func TestImportAndRenderVideoOnce(t *testing.T) {
	_, markdown, _, _ := importAndRenderObject(t, readFixture(t, "video.json"),
		&fakeRequester{picAvailable: true})
	const videoURL = "https://video.weibo.com/show?fid=1034:5310847304532009"
	if count := strings.Count(markdown, videoURL); count != 1 {
		t.Fatalf("video appears %d times:\n%s", count, markdown)
	}
}

func TestImportAndRenderRetweetWithOriginalAndTime(t *testing.T) {
	_, original, markdown, db, _ := importAndRenderPair(t, "retweet_with_original.json",
		&fakeRequester{picAvailable: true})
	if !strings.Contains(markdown, "> 转发 @"+original.ScreenName) ||
		!strings.Contains(markdown, "> 2026 年 06 月 08 日 08:55") {
		t.Fatalf("retweet content/time missing:\n%s", markdown)
	}
	if db.posts[original.ID].InTimeline {
		t.Fatal("embedded original must not be a timeline entry")
	}
}

func TestImportAndRenderRetweetUsesNestedOriginalWhenSeparateObjectAbsent(t *testing.T) {
	var fixture struct {
		Repost json.RawMessage `json:"repost"`
	}
	if err := json.Unmarshal(readFixture(t, "retweet_original_absent.json"), &fixture); err != nil {
		t.Fatal(err)
	}
	_, markdown, _, _ := importAndRenderObject(t, fixture.Repost,
		&fakeRequester{picAvailable: true})
	if !strings.Contains(markdown, "> 转发 @") {
		t.Fatalf("nested original was not rendered:\n%s", markdown)
	}
}

func TestImportAndRenderH5ImageBeforeRetweet(t *testing.T) {
	repostSource, _ := loadRetweetPair(t, "view_pic_retweet.json")
	longURL := repostSource.Links[0].LongURL
	const h5PicID = "53899d01ly1ief0r5kg95j210o2q6npd"
	_, _, markdown, _, _ := importAndRenderPair(t, "view_pic_retweet.json", &fakeRequester{
		picAvailable: true,
		reppics:      map[string][]string{longURL: {h5PicID}},
	})
	imageURL := "https://oss.test/rss/tombkeeper/" + h5PicID + ".jpg"
	embed := strings.Index(markdown, "![微博图片 1]("+imageURL+")")
	quote := strings.Index(markdown, "> 转发 @")
	if embed < 0 || quote < 0 || embed > quote {
		t.Fatalf("H5 image must appear before retweet quote:\n%s", markdown)
	}
}

func TestImportAndRenderH5FailureFallsBackToOriginalLink(t *testing.T) {
	repostSource, _ := loadRetweetPair(t, "view_pic_retweet.json")
	longURL := repostSource.Links[0].LongURL
	_, _, markdown, _, _ := importAndRenderPair(t, "view_pic_retweet.json", &fakeRequester{
		picAvailable: true,
		reppicErr:    true,
	})
	// autocorrect-disable（该标签属于既定输出）
	want := "[查看图片|原始链接](" + longURL + ")"
	// autocorrect-enable
	if !strings.Contains(markdown, want) {
		t.Fatalf("missing H5 fallback %q:\n%s", want, markdown)
	}
}

func loadRetweetPair(t *testing.T, name string) (repost, original SourcePost) {
	t.Helper()
	var fixture struct {
		Repost   json.RawMessage `json:"repost"`
		Original json.RawMessage `json:"original"`
	}
	if err := json.Unmarshal(readFixture(t, name), &fixture); err != nil {
		t.Fatalf("unmarshal %s: %v", name, err)
	}
	var err error
	if repost, err = parseSourcePost(fixture.Repost, nil); err != nil {
		t.Fatalf("parse repost: %v", err)
	}
	if original, err = parseSourcePost(fixture.Original, nil); err != nil {
		t.Fatalf("parse original: %v", err)
	}
	return repost, original
}
