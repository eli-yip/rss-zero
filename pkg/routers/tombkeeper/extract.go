package tombkeeper

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

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

// ExtractPosts parses all weibo objects out of a tombkeeper.io page, in document
// order, de-duplicated by id.
func ExtractPosts(html []byte) ([]RawPost, error) {
	flight := reassembleFlight(html)
	rows := parseFlightRows(flight)

	var posts []RawPost
	seen := make(map[string]struct{})
	for _, obj := range extractObjects(flight) {
		p, err := parseRawPost([]byte(obj), rows)
		if err != nil {
			continue // tolerate a malformed object, keep the rest of the page
		}
		if _, dup := seen[p.ID]; dup {
			continue
		}
		seen[p.ID] = struct{}{}
		posts = append(posts, p)
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

// parseRawPost decodes one object substring into a RawPost, resolving the
// url_info $ref against rows and parsing the $D-prefixed created_at.
func parseRawPost(obj []byte, rows map[string]string) (RawPost, error) {
	var w rawPostWire
	if err := json.Unmarshal(obj, &w); err != nil {
		return RawPost{}, err
	}

	raw := make([]byte, len(obj))
	copy(raw, obj)

	return RawPost{
		ID:         w.ID,
		Bid:        w.Bid,
		UserID:     w.UserID,
		ScreenName: w.ScreenName,
		Text:       resolveFlightString(w.Text, rows),
		Pics:       w.Pics,
		CreatedAt:  parseFlightTime(w.CreatedAt),
		RetweetID:  w.RetweetID,
		URLInfo:    resolveURLInfo(w.URLInfo, rows),
		Raw:        raw,
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

// resolveURLInfo turns the raw url_info value into entries. It accepts either an
// inline array (fixtures) or a "$<rowid>" reference resolved against rows.
func resolveURLInfo(raw json.RawMessage, rows map[string]string) []URLInfoEntry {
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

	var entries []URLInfoEntry
	if err := json.Unmarshal([]byte(arrayJSON), &entries); err != nil {
		return nil
	}
	return entries
}
