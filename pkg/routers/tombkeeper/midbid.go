package tombkeeper

import (
	"fmt"
	"strconv"
	"strings"
)

// base62Alphabet is weibo's base62 alphabet for mid<->mblogid conversion.
// IMPORTANT: the order is digits, then LOWERCASE, then UPPERCASE — using
// uppercase-before-lowercase produces wrong results. The scheme is verified
// bidirectional and lossless against real (mid, bid) pairs (see midbid_test.go).
const base62Alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var base62Index = func() map[byte]int {
	m := make(map[byte]int, len(base62Alphabet))
	for i := range len(base62Alphabet) {
		m[base62Alphabet[i]] = i
	}
	return m
}()

func base62Encode(n uint64) string {
	if n == 0 {
		return "0"
	}
	var buf []byte
	for n > 0 {
		buf = append(buf, base62Alphabet[n%62])
		n /= 62
	}
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf)
}

func base62Decode(s string) (uint64, error) {
	var n uint64
	for i := range len(s) {
		v, ok := base62Index[s[i]]
		if !ok {
			return 0, fmt.Errorf("invalid base62 char %q", s[i])
		}
		n = n*62 + uint64(v)
	}
	return n, nil
}

// MidToBid converts a numeric weibo mid (decimal string) to its mblogid (bid).
// The mid is split into groups of 7 digits from the right; each group is
// base62-encoded and every group except the leftmost is left-padded with '0'
// to 4 characters before concatenation.
func MidToBid(mid string) string {
	var parts []string
	for end := len(mid); end > 0; {
		start := max(end-7, 0)
		n, _ := strconv.ParseUint(mid[start:end], 10, 64)
		enc := base62Encode(n)
		if start != 0 {
			enc = strings.Repeat("0", 4-len(enc)) + enc
		}
		parts = append(parts, enc)
		end = start
	}
	var b strings.Builder
	for i := len(parts) - 1; i >= 0; i-- {
		b.WriteString(parts[i])
	}
	return b.String()
}

// BidToMid converts an mblogid (bid) back to the numeric weibo mid (decimal
// string). The bid is split into groups of 4 chars from the right; each group
// is base62-decoded and every group except the leftmost is left-padded with
// '0' to 7 digits before concatenation. Returns an error on invalid characters.
func BidToMid(bid string) (string, error) {
	if bid == "" {
		return "", fmt.Errorf("empty bid")
	}
	var parts []string
	for end := len(bid); end > 0; {
		start := max(end-4, 0)
		n, err := base62Decode(bid[start:end])
		if err != nil {
			return "", err
		}
		s := strconv.FormatUint(n, 10)
		if start != 0 {
			// A valid non-leftmost group is one 7-digit decimal block, so it must
			// be < 10^7. A 4-char base62 group can decode up to 62^4-1 = 14776335
			// (8 digits); such a value is not a real bid and would make the pad
			// count negative (panic), so reject it.
			if len(s) > 7 {
				return "", fmt.Errorf("invalid bid %q: group %q out of range", bid, bid[start:end])
			}
			s = strings.Repeat("0", 7-len(s)) + s
		}
		parts = append(parts, s)
		end = start
	}
	var b strings.Builder
	for i := len(parts) - 1; i >= 0; i-- {
		b.WriteString(parts[i])
	}
	return b.String(), nil
}
