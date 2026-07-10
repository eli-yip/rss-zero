package tkblog

import (
	"encoding/json"
	"fmt"
	"testing"
)

// pushChunk wraps a flight substring as one self.__next_f.push([1,"…"]) call,
// JSON-escaping the content exactly as the page does.
func pushChunk(s string) string {
	b, _ := json.Marshal(s)
	return `self.__next_f.push([1,` + string(b) + `])`
}

func TestExtractArticles(t *testing.T) {
	// refBody stands in for a content row referenced as "$17". It carries an
	// internal "\n12:" that mimics a row delimiter, so the T<hexlen> byte length
	// must be honored or the content would truncate.
	refBody := "第一段\n12:34 见\n\n第二段"

	// a1: inline content, baidu. a2: title AND content given as $refs, xfocus.
	a1 := `{"id":"26ho1i8FcjS","category_slug":"baidu","title":"猕猴桃的科学",` +
		`"created_at":"$D2011-10-09T13:25:00.000Z",` +
		`"url":"https://web.archive.org/web/2011id_/http://hi.baidu.com/x.html",` +
		`"content":"正文一\n\n正文二"}`
	a2 := `{"id":"9kZ2xFocus","category_slug":"xfocus","title":"$18",` +
		`"created_at":"$D2008-05-01T00:00:00.000Z",` +
		`"url":"https://web.archive.org/web/2008id_/http://blog.xfocus.net/y.html",` +
		`"content":"$17"}`
	row17 := fmt.Sprintf("\n17:T%x,%s\n", len(refBody), refBody)
	row18 := "\n18:\"标题引用\"\n"

	flight := "9:[" + a1 + "," + a2 + "]" + row17 + row18 + `10:{"totalPages":33}` + "\n"

	// Split the flight across two chunks at an arbitrary midpoint to exercise
	// cross-chunk reassembly.
	mid := len(flight) / 2
	html := []byte(pushChunk(flight[:mid]) + "<script></script>" + pushChunk(flight[mid:]))

	arts, total, err := ExtractArticles(html)
	if err != nil {
		t.Fatalf("ExtractArticles error: %v", err)
	}
	if total != 33 {
		t.Fatalf("totalPages = %d, want 33", total)
	}
	if len(arts) != 2 {
		t.Fatalf("got %d articles, want 2", len(arts))
	}

	// a1: inline fields, $D date parsed, order preserved (first).
	if arts[0].ID != "26ho1i8FcjS" || arts[0].Category != "baidu" {
		t.Errorf("a1 id/category = %q/%q", arts[0].ID, arts[0].Category)
	}
	if arts[0].Title != "猕猴桃的科学" {
		t.Errorf("a1 title = %q", arts[0].Title)
	}
	if arts[0].CreatedAt.Year() != 2011 || arts[0].CreatedAt.Month() != 10 {
		t.Errorf("a1 created_at = %v", arts[0].CreatedAt)
	}
	if arts[0].Content != "正文一\n\n正文二" {
		t.Errorf("a1 content = %q", arts[0].Content)
	}
	if arts[0].URL != "https://web.archive.org/web/2011id_/http://hi.baidu.com/x.html" {
		t.Errorf("a1 url = %q", arts[0].URL)
	}

	// a2: title resolved from $18 (a quoted-string row), content from $17 (a T row
	// whose internal "\n12:" must not truncate it).
	if arts[1].ID != "9kZ2xFocus" || arts[1].Category != "xfocus" {
		t.Errorf("a2 id/category = %q/%q", arts[1].ID, arts[1].Category)
	}
	if arts[1].Title != "标题引用" {
		t.Errorf("a2 title (ref) = %q, want 标题引用", arts[1].Title)
	}
	if arts[1].Content != refBody {
		t.Errorf("a2 content (ref) = %q, want %q", arts[1].Content, refBody)
	}
}

// A malformed article object (broken JSON) is skipped, the rest of the page kept.
func TestExtractArticlesToleratesMalformed(t *testing.T) {
	good := `{"id":"good1","category_slug":"baidu","title":"ok","created_at":"$D2011-01-01T00:00:00.000Z","url":"","content":"body"}`
	// Balanced braces (so it doesn't disturb `good`), but invalid JSON: "title":,
	// has no value, so json.Unmarshal fails and the object is skipped.
	bad := `{"id":"bad1","category_slug":"baidu","title":,"created_at":"x","url":"","content":"z"}`
	flight := "9:[" + bad + "," + good + `]10:{"totalPages":1}` + "\n"

	arts, total, err := ExtractArticles([]byte(pushChunk(flight)))
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 {
		t.Fatalf("totalPages = %d, want 1", total)
	}
	if len(arts) != 1 || arts[0].ID != "good1" {
		t.Fatalf("got %+v, want only good1", arts)
	}
}
