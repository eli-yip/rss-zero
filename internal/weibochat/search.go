package weibochat

import (
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// SearchQuery matches every literal term. Quoted phrases are single terms.
// Only ASCII letters are case-insensitive, consistently with the SQL expression.
type SearchQuery struct{ Terms []string }

// TextMatch is a half-open range of Unicode code points in the original text.
type TextMatch struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

func ParseSearchQuery(input string) (SearchQuery, error) {
	result := SearchQuery{Terms: []string{}}
	if utf8.RuneCountInString(input) > 256 {
		return result, fmt.Errorf("search query must not exceed 256 Unicode code points")
	}
	runes := []rune(input)
	var term []rune
	quoted, quoteStart := false, 0
	flush := func() {
		if len(term) > 0 {
			result.Terms = append(result.Terms, string(term))
			term = nil
		}
	}
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		if r == '\\' && i+1 < len(runes) && (runes[i+1] == '\\' || runes[i+1] == '"') {
			i++
			term = append(term, runes[i])
			continue
		}
		if r == '"' {
			if quoted && len(term) == quoteStart {
				return result, fmt.Errorf("search phrase must not be empty")
			}
			quoted = !quoted
			quoteStart = len(term)
			continue
		}
		if !quoted && unicode.IsSpace(r) {
			flush()
			continue
		}
		term = append(term, r)
	}
	if quoted {
		return result, fmt.Errorf("search phrase has an unclosed double quote")
	}
	flush()
	return result, nil
}

func foldSearchText(text string) string {
	return strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}, text)
}

// FindMatches highlights individual terms, even when the full AND query does not
// match; highlight-only queries are also used on surrounding conversations.
func (q SearchQuery) FindMatches(text string) []TextMatch {
	matches := []TextMatch{}
	body := []rune(foldSearchText(text))
	for _, term := range q.Terms {
		needle := []rune(foldSearchText(term))
		if len(needle) == 0 {
			continue
		}
		for start := 0; start+len(needle) <= len(body); start++ {
			if slices.Equal(body[start:start+len(needle)], needle) {
				matches = append(matches, TextMatch{Start: start, End: start + len(needle)})
			}
		}
	}
	slices.SortFunc(matches, func(a, b TextMatch) int {
		if a.Start != b.Start {
			return a.Start - b.Start
		}
		return a.End - b.End
	})
	merged := make([]TextMatch, 0, len(matches))
	for _, match := range matches {
		if len(merged) > 0 && match.Start < merged[len(merged)-1].End {
			merged[len(merged)-1].End = max(merged[len(merged)-1].End, match.End)
		} else {
			merged = append(merged, match)
		}
	}
	return merged
}

func searchPattern(term string) string {
	return "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(foldSearchText(term)) + "%"
}
