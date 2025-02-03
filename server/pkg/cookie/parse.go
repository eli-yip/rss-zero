package cookie

import (
	"strings"
	"time"
)

func ParseArcExpireAt(expireAt string) (time.Time, error) {
	// Sat Jul 27 2024 16:48:02 GMT+0800
	expireAt = strings.TrimSuffix(expireAt, "(中国标准时间)")
	expireAt = strings.TrimSpace(expireAt)
	const layout = "Mon Jan 02 2006 15:04:05 GMT-0700"
	return time.Parse(layout, expireAt)
}

func ExtractCookieValue(cookie, cookieName string) (result string) {
	cookie = strings.TrimSpace(cookie)
	cookie = strings.TrimSuffix(cookie, ";")
	return strings.TrimPrefix(cookie, cookieName+"=")
}
