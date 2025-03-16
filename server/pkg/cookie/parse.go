package cookie

import (
	"fmt"
	"strings"
	"time"
)

func ParseArcExpireAt(expireAt any) (time.Time, error) {
	switch v := expireAt.(type) {
	case string:
		// Sat Jul 27 2024 16:48:02 GMT+0800
		v = strings.TrimSuffix(v, "(中国标准时间)")
		v = strings.TrimSpace(v)
		const layout = "Mon Jan 02 2006 15:04:05 GMT-0700"
		return time.Parse(layout, v)
	case float64:
		return time.Unix(int64(v), 0), nil
	default:
		return time.Time{}, fmt.Errorf("invalid expire_at type: %T", expireAt)
	}
}

func ExtractCookieValue(cookie, cookieName string) (result string) {
	cookie = strings.TrimSpace(cookie)
	cookie = strings.TrimSuffix(cookie, ";")
	return strings.TrimPrefix(cookie, cookieName+"=")
}
