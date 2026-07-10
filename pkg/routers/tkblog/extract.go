package tkblog

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// tombkeeper.io serves its xfocus / baidu blog pages from the same Next.js SSR
// template as the weibo timeline: each article's structured data lives in the
// page's RSC flight payload, split across self.__next_f.push([1,"…"]) chunks. An
// article object is {"id","category_slug","title","created_at":"$D…","url","content"};
// content may be a "$<row>" reference and created_at carries the $D date marker.
//
// ponytail: the flight machinery below (reassembleFlight / resolveFlightString /
// matchBraces / parseFlightTime) is copied from tombkeeper/extract.go rather than
// shared. Abstracting it would force a rewrite-and-retest of the golden-tested
// weibo path for no functional gain; hoist to internal/nextflight if a third
// Next.js source appears.
//
// parseFlightRows is DELIBERATELY NOT copied: the weibo version finds row starts
// with a `(?:^|\n)<hexid>:` regex, which assumes every row is newline-preceded.
// The blog pages chain text rows back-to-back with no separator — a T row's byte
// count runs straight into the next `<hexid>:` — so the regex misses every row
// that follows a T row (exactly the $ref content rows). This version walks the row
// stream sequentially instead, using each T row's declared byte length as the
// delimiter to the next row.

var (
	nextFChunkRe = regexp.MustCompile(`self\.__next_f\.push\(\[1,"((?:[^"\\]|\\.)*)"\]\)`)
	// An article object starts {"id":"<token>","category_slug":… — anchoring on the
	// id→category_slug shape (tolerating reformat whitespace) keeps it from matching
	// wrapper objects that merely embed an article, and from matching the weibo
	// objects (numeric id, no category_slug).
	objStartRe   = regexp.MustCompile(`\{\s*"id"\s*:\s*"[0-9A-Za-z]+"\s*,\s*"category_slug"`)
	totalPagesRe = regexp.MustCompile(`"totalPages"\s*:\s*(\d+)`)
)

// RawArticle is one blog article extracted from a page's RSC flight data, with the
// $D date marker parsed and any content/title $ref resolved.
type RawArticle struct {
	ID        string
	Category  string // "xfocus" or "baidu"
	Title     string
	CreatedAt time.Time
	URL       string // Wayback Machine original link
	Content   string // plain text, not HTML
}

// ExtractArticles parses every blog article out of a tombkeeper.io list page, in
// document order, de-duplicated by (category, id), and reads the page's totalPages.
// A malformed article object is skipped, keeping the rest of the page.
func ExtractArticles(html []byte) (arts []RawArticle, totalPages int, err error) {
	flight := reassembleFlight(html)
	rows := parseFlightRows(flight)

	if m := totalPagesRe.FindStringSubmatch(flight); m != nil {
		totalPages, _ = strconv.Atoi(m[1])
	}

	seen := make(map[string]struct{})
	for _, obj := range extractObjects(flight) {
		a, ok := parseRawArticle([]byte(obj), rows)
		if !ok {
			continue // tolerate a malformed object, keep the rest of the page
		}
		key := a.Category + "/" + a.ID
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		arts = append(arts, a)
	}
	return arts, totalPages, nil
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

func isHexByte(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
}

// parseFlightRows maps each flight row id (e.g. "16") to its raw value text, used
// to resolve $ref references such as a content body carried as "$16". It walks the
// row stream sequentially: a text row "T<hexlen>,<text>" is delimited by its
// declared byte length (so internal newlines don't truncate it, and a following
// row packed with no separator is still found); every other row runs to the next
// newline (RSC serializes non-text rows on a single line).
func parseFlightRows(flight string) map[string]string {
	rows := make(map[string]string)
	n := len(flight)
	for i := 0; i < n; {
		// Read a row id: up to 4 hex chars followed by ':'.
		j := i
		for j < n && j-i < 4 && isHexByte(flight[j]) {
			j++
		}
		if j == i || j >= n || flight[j] != ':' {
			// Not a row start here (flight prefix, or desync): skip to the next line.
			nl := strings.IndexByte(flight[i:], '\n')
			if nl < 0 {
				break
			}
			i += nl + 1
			continue
		}

		id := flight[i:j]
		valStart := j + 1
		// Text row "T<hexlen>,<text>": consume exactly hexlen bytes; the next row
		// begins immediately after (optionally past a single trailing newline).
		if valStart < n && flight[valStart] == 'T' {
			if comma := strings.IndexByte(flight[valStart:], ','); comma > 0 {
				if length, err := strconv.ParseInt(flight[valStart+1:valStart+comma], 16, 64); err == nil {
					textStart := valStart + comma + 1
					if end := textStart + int(length); end <= n {
						rows[id] = flight[valStart:end]
						i = end
						if i < n && flight[i] == '\n' {
							i++
						}
						continue
					}
				}
			}
		}
		// Non-text row: value runs to the next newline.
		nl := strings.IndexByte(flight[valStart:], '\n')
		if nl < 0 {
			rows[id] = strings.TrimSpace(flight[valStart:])
			break
		}
		rows[id] = strings.TrimSpace(flight[valStart : valStart+nl])
		i = valStart + nl + 1
	}
	return rows
}

var flightStringRefRe = regexp.MustCompile(`^\$[0-9a-f]+$`)

// resolveFlightString resolves a possibly-referenced flight string value: a "$$…"
// escape yields the literal "$…"; a "$<rowid>" reference is resolved against rows
// (text rows are "T<hexlen>,<text>"); anything else is returned as-is. Next.js
// encodes a literal leading '$' as '$$', so a lone "$<hex>" is always a reference.
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

// extractObjects returns the brace-matched substring of every article object
// (objects opening {"id":"<token>","category_slug":…).
func extractObjects(flight string) []string {
	var objs []string
	for _, loc := range objStartRe.FindAllStringIndex(flight, -1) {
		end, ok := matchBraces(flight, loc[0])
		if !ok {
			continue
		}
		objs = append(objs, flight[loc[0]:end+1])
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

type rawArticleWire struct {
	ID           string `json:"id"`
	CategorySlug string `json:"category_slug"`
	Title        string `json:"title"`
	CreatedAt    string `json:"created_at"`
	URL          string `json:"url"`
	Content      string `json:"content"`
}

// parseRawArticle decodes one object substring into a RawArticle, resolving the
// $D-prefixed created_at and any $ref-carried title/url/content. ok is false for a
// malformed object or one missing id/category_slug.
func parseRawArticle(obj []byte, rows map[string]string) (RawArticle, bool) {
	var w rawArticleWire
	if err := json.Unmarshal(obj, &w); err != nil {
		return RawArticle{}, false
	}
	if w.ID == "" || w.CategorySlug == "" {
		return RawArticle{}, false
	}
	return RawArticle{
		ID:        w.ID,
		Category:  w.CategorySlug,
		Title:     resolveFlightString(w.Title, rows),
		CreatedAt: parseFlightTime(w.CreatedAt),
		URL:       resolveFlightString(w.URL, rows),
		Content:   resolveFlightString(w.Content, rows),
	}, true
}

// parseFlightTime strips the Next.js "$D" date marker and parses RFC3339 (UTC).
func parseFlightTime(s string) time.Time {
	s = strings.TrimPrefix(s, "$D")
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}
