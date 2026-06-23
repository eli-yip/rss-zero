package tombkeeper

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// pushChunk wraps a flight substring as one self.__next_f.push([1,"…"]) call,
// JSON-escaping the content exactly as the page does.
func pushChunk(s string) string {
	b, _ := json.Marshal(s)
	return `self.__next_f.push([1,` + string(b) + `])`
}

func TestExtractPostsCrossChunkAndRef(t *testing.T) {
	// A post whose text contains a quote and braces (must not break brace
	// matching), url_info given as a $ref, created_at carrying the $D marker.
	post := `{"id":"5312623991326758","bid":"R5iph5z9k","user_id":"1401527553",` +
		`"screen_name":"tombkeeper","text":"a\"b {c} hello\nworld #tag#",` +
		`"pics":"abc123,def456","video_url":"","created_at":"$D2026-06-22T07:02:06.000Z",` +
		`"retweet_id":"","url_info":"$1f"}`
	row := "\n1f:[{\"short_url\":\"http://t.cn/X\",\"weibo_bid\":\"R5iph5z9k\"," +
		"\"url_type\":0,\"url_title\":\"微博正文\",\"long_url\":\"https://weibo.com/1401527553/R4FtclSR7\"}]\n"

	flight := "preamble\n9:" + post + row + "10:[]\n"

	// Split the flight across two chunks at an arbitrary midpoint to exercise
	// cross-chunk reassembly (the '{' and '}' may land in different chunks).
	mid := len(flight) / 2
	html := []byte(pushChunk(flight[:mid]) + "<script></script>" + pushChunk(flight[mid:]))

	posts, err := ExtractPosts(html)
	if err != nil {
		t.Fatalf("ExtractPosts error: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("got %d posts, want 1", len(posts))
	}
	p := posts[0]
	if p.ID != "5312623991326758" {
		t.Errorf("id = %q", p.ID)
	}
	if p.Pics != "abc123,def456" {
		t.Errorf("pics = %q", p.Pics)
	}
	if !strings.Contains(p.Text, "hello\nworld") || !strings.Contains(p.Text, "{c}") {
		t.Errorf("text = %q", p.Text)
	}
	if p.CreatedAt.Year() != 2026 || p.CreatedAt.Month() != 6 {
		t.Errorf("created_at = %v", p.CreatedAt)
	}
	if len(p.URLInfo) != 1 || p.URLInfo[0].URLTitle != "微博正文" ||
		p.URLInfo[0].LongURL != "https://weibo.com/1401527553/R4FtclSR7" {
		t.Errorf("url_info = %+v", p.URLInfo)
	}
}

// Some posts carry text as a flight reference ("text":"$17") rather than inline,
// pointing at a "T<hexlen>,<text>" row. The byte length must be used so an
// internal newline that looks like a row delimiter does not truncate the text.
func TestExtractResolvesTextRef(t *testing.T) {
	body := "价格\n18:30 见" // contains a "\n18:" that mimics a row delimiter
	post := `{"id":"5312949716521436","bid":"R5qSDrmws","user_id":"1401527553",` +
		`"screen_name":"tombkeeper","text":"$17","pics":"","video_url":"",` +
		`"created_at":"$D2026-06-23T04:36:25.000Z","retweet_id":"","url_info":[]}`
	row := fmt.Sprintf("\n17:T%x,%s\n", len(body), body)
	flight := "9:" + post + row + "1f:[]\n"

	posts, err := ExtractPosts([]byte(pushChunk(flight)))
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 {
		t.Fatalf("got %d posts, want 1", len(posts))
	}
	if posts[0].Text != body {
		t.Errorf("text = %q, want %q (ref not resolved / truncated)", posts[0].Text, body)
	}
}

func TestExtractPostsToleratesWhitespace(t *testing.T) {
	// A pretty-printed/reformatted object (whitespace after '{' and around ':')
	// must still be extracted — objStartRe tolerates whitespace around the id key.
	post := `{ "id" : "200", "bid":"B","user_id":"1401527553","screen_name":"tombkeeper",` +
		`"text":"x","pics":"","video_url":"","created_at":"$D2026-01-02T03:04:05.000Z",` +
		`"retweet_id":"","url_info":[]}`
	posts, err := ExtractPosts([]byte(pushChunk("9:" + post + "\n")))
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 || posts[0].ID != "200" {
		t.Fatalf("whitespaced object not extracted: %+v", posts)
	}
}

func TestExtractPostsInlineURLInfoAndDedup(t *testing.T) {
	post := `{"id":"100","bid":"B","user_id":"1401527553","screen_name":"tombkeeper",` +
		`"text":"x","pics":"","video_url":"","created_at":"$D2026-01-02T03:04:05.000Z",` +
		`"retweet_id":"","url_info":[{"short_url":"s","url_type":0,"url_title":"t","long_url":"l"}]}`
	// Same object appears twice; dedup should keep one.
	flight := "9:" + post + "\na:" + post + "\n"
	html := []byte(pushChunk(flight))

	posts, err := ExtractPosts(html)
	if err != nil {
		t.Fatalf("ExtractPosts error: %v", err)
	}
	if len(posts) != 1 {
		t.Fatalf("got %d posts, want 1 (dedup)", len(posts))
	}
	if len(posts[0].URLInfo) != 1 || posts[0].URLInfo[0].LongURL != "l" {
		t.Errorf("inline url_info not parsed: %+v", posts[0].URLInfo)
	}
}
