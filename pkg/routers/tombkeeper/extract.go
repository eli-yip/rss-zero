package tombkeeper

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ExtractTimelinePage 汇合 flight 博文对象与 SSR「详情」链接，返回分类后的页面内容。
func ExtractTimelinePage(html []byte) (TimelinePage, error) {
	sourcePosts, err := extractSourcePosts(html)
	if err != nil {
		return TimelinePage{}, err
	}
	ids := timelineIDs(html)
	if len(ids) == 0 && len(sourcePosts) > 0 {
		return TimelinePage{}, fmt.Errorf("flight has %d posts but no timeline detail links", len(sourcePosts))
	}

	posts := make(map[int64]SourcePost, len(sourcePosts))
	order := make([]int64, 0, len(sourcePosts))
	for _, post := range sourcePosts {
		posts[post.ID] = post
		order = append(order, post.ID)
	}

	page := TimelinePage{}
	entries := make(map[int64]struct{}, len(ids))
	for _, rawID := range ids {
		id, err := strconv.ParseInt(rawID, 10, 64)
		if err != nil {
			page.MissingEntries++
			continue
		}
		entries[id] = struct{}{}
		post, ok := posts[id]
		if !ok {
			page.MissingEntries++
			continue
		}
		page.Entries = append(page.Entries, post)
	}
	for _, id := range order {
		if _, isEntry := entries[id]; isEntry {
			continue
		}
		page.EmbeddedPosts = append(page.EmbeddedPosts, posts[id])
	}
	return page, nil
}

// tombkeeper.io is a Next.js SSR site: each weibo's structured data is embedded
// in the page's RSC flight payload, split across many self.__next_f.push([1,"…"])
// chunks. Extraction reassembles the chunks, brace-matches the post objects, and
// resolves the $D date marker and the url_info $ref. See example/README.md §1.

var (
	nextFChunkRe = regexp.MustCompile(`self\.__next_f\.push\(\[1,"((?:[^"\\]|\\.)*)"\]\)`)
	flightRowRe  = regexp.MustCompile(`(?:^|\n)([0-9a-f]{1,4}):`)
	// Tolerate whitespace around the first key so a pretty-printed/reformatted
	// flight object is still matched (the value's leading digit keeps it specific
	// to post objects, which always carry a numeric id).
	objStartRe = regexp.MustCompile(`\{\s*"id"\s*:\s*"\d`)
)

// extractSourcePosts 按文档顺序解析全部博文，并按 id 去重。
func extractSourcePosts(html []byte) ([]SourcePost, error) {
	flight := reassembleFlight(html)
	rows := parseFlightRows(flight)

	var posts []SourcePost
	seen := make(map[int64]struct{})
	for _, obj := range extractObjects(flight) {
		post, err := parseSourcePost([]byte(obj), rows)
		if err != nil {
			continue // tolerate a malformed object, keep the rest of the page
		}
		if _, dup := seen[post.ID]; dup {
			continue
		}
		seen[post.ID] = struct{}{}
		posts = append(posts, post)
	}
	return posts, nil
}

// reassembleFlight concatenates all __next_f.push payload chunks (JSON-unescaping
// each) into the full RSC flight string.
func reassembleFlight(html []byte) string {
	var b strings.Builder
	for _, m := range nextFChunkRe.FindAllSubmatch(html, -1) {
		var s string
		if err := json.Unmarshal([]byte(`"`+string(m[1])+`"`), &s); err != nil {
			continue // skip an undecodable chunk
		}
		b.WriteString(s)
	}
	return b.String()
}

// parseFlightRows maps each flight row id (e.g. "1f") to its raw value text, used
// to resolve $ref references such as url_info.
func parseFlightRows(flight string) map[string]string {
	rows := make(map[string]string)
	locs := flightRowRe.FindAllStringSubmatchIndex(flight, -1)
	for i, loc := range locs {
		id := flight[loc[2]:loc[3]]
		valStart := loc[1]
		// Text rows are "T<hexlen>,<text>": use the declared byte length to capture
		// the exact text, whose internal newlines could otherwise look like a row
		// delimiter and truncate it.
		if valStart < len(flight) && flight[valStart] == 'T' {
			if comma := strings.IndexByte(flight[valStart:], ','); comma > 0 {
				if n, err := strconv.ParseInt(flight[valStart+1:valStart+comma], 16, 64); err == nil {
					if end := valStart + comma + 1 + int(n); end <= len(flight) {
						rows[id] = flight[valStart : valStart+comma+1+int(n)]
						continue
					}
				}
			}
		}
		valEnd := len(flight)
		if i+1 < len(locs) {
			valEnd = locs[i+1][0]
		}
		rows[id] = strings.TrimSpace(flight[valStart:valEnd])
	}
	return rows
}

var flightStringRefRe = regexp.MustCompile(`^\$[0-9a-f]+$`)

// resolveFlightString resolves a possibly-referenced flight string value: a
// "$$…" escape yields the literal "$…"; a "$<rowid>" reference is resolved
// against rows (text rows are "T<hexlen>,<text>"); anything else is returned
// as-is. (Next.js encodes a literal leading '$' as '$$', so a lone "$<hex>" is
// always a reference, e.g. text:"$17".)
func resolveFlightString(s string, rows map[string]string) string {
	if strings.HasPrefix(s, "$$") {
		return s[1:]
	}
	if !flightStringRefRe.MatchString(s) {
		return s
	}
	row := rows[s[1:]]
	if strings.HasPrefix(row, "T") {
		if comma := strings.IndexByte(row, ','); comma > 0 {
			return row[comma+1:]
		}
	}
	if strings.HasPrefix(row, `"`) {
		var str string
		if json.Unmarshal([]byte(row), &str) == nil {
			return str
		}
	}
	return ""
}

// extractObjects returns the brace-matched substrings of every weibo post object
// (objects starting with {"id":"<digit> and carrying created_at + retweet_id).
func extractObjects(flight string) []string {
	var objs []string
	for _, loc := range objStartRe.FindAllStringIndex(flight, -1) {
		end, ok := matchBraces(flight, loc[0])
		if !ok {
			continue
		}
		obj := flight[loc[0] : end+1]
		if strings.Contains(obj, `"retweet_id"`) && strings.Contains(obj, `"created_at"`) {
			objs = append(objs, obj)
		}
	}
	return objs
}

// matchBraces returns the index of the '}' that closes the '{' at start, honoring
// string literals and escapes.
func matchBraces(s string, start int) (int, bool) {
	depth, inStr, esc := 0, false, false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i, true
			}
		}
	}
	return 0, false
}

type rawPostWire struct {
	ID         string          `json:"id"`
	Bid        string          `json:"bid"`
	UserID     string          `json:"user_id"`
	ScreenName string          `json:"screen_name"`
	Text       string          `json:"text"`
	Pics       string          `json:"pics"`
	CreatedAt  string          `json:"created_at"`
	RetweetID  string          `json:"retweet_id"`
	URLInfo    json.RawMessage `json:"url_info"`
}

// parseSourcePost 解析单个对象，并解开 url_info 引用与 $D 时间。
func parseSourcePost(obj []byte, rows map[string]string) (SourcePost, error) {
	var w rawPostWire
	if err := json.Unmarshal(obj, &w); err != nil {
		return SourcePost{}, err
	}
	id, err := strconv.ParseInt(w.ID, 10, 64)
	if err != nil {
		return SourcePost{}, fmt.Errorf("parse post id %q: %w", w.ID, err)
	}
	publishedAt := parseFlightTime(w.CreatedAt)
	if publishedAt.IsZero() {
		return SourcePost{}, fmt.Errorf("parse post %d created_at %q", id, w.CreatedAt)
	}
	var retweetID int64
	if w.RetweetID != "" {
		retweetID, err = strconv.ParseInt(w.RetweetID, 10, 64)
		if err != nil {
			return SourcePost{}, fmt.Errorf("parse retweet id %q: %w", w.RetweetID, err)
		}
	}
	return SourcePost{
		ID:            id,
		Bid:           w.Bid,
		AuthorID:      w.UserID,
		ScreenName:    w.ScreenName,
		Text:          resolveFlightString(w.Text, rows),
		Pics:          splitPics(w.Pics),
		PublishedAt:   publishedAt,
		RetweetPostID: retweetID,
		Links:         resolvePostLinks(w.URLInfo, rows),
	}, nil
}

// parseFlightTime strips the Next.js "$D" date marker and parses RFC3339.
func parseFlightTime(s string) time.Time {
	s = strings.TrimPrefix(s, "$D")
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// resolvePostLinks 解析内联数组或指向 flight row 的 url_info。
func resolvePostLinks(raw json.RawMessage, rows map[string]string) []PostLink {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil
	}

	arrayJSON := trimmed
	if trimmed[0] == '"' {
		var ref string
		if err := json.Unmarshal(raw, &ref); err != nil {
			return nil
		}
		if !strings.HasPrefix(ref, "$") {
			return nil
		}
		arrayJSON = rows[strings.TrimPrefix(ref, "$")]
	}
	if !strings.HasPrefix(arrayJSON, "[") {
		return nil
	}

	var entries []PostLink
	if err := json.Unmarshal([]byte(arrayJSON), &entries); err != nil {
		return nil
	}
	return entries
}
