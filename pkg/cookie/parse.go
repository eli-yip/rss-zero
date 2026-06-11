package cookie

import "strings"

// ExtractCookieValue strips a leading "name=" prefix (and a trailing ";") from a
// raw cookie string, returning just the value. A bare value passes through unchanged.
func ExtractCookieValue(cookie, cookieName string) (result string) {
	cookie = strings.TrimSpace(cookie)
	cookie = strings.TrimSuffix(cookie, ";")
	return strings.TrimPrefix(cookie, cookieName+"=")
}
