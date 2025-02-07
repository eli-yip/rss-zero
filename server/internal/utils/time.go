package utils

import (
	"time"

	"github.com/eli-yip/rss-zero/config"
)

func NilToEmpty(str *string) string {
	if str == nil {
		return ""
	}
	return *str
}

func UnixToTime(unix int64) time.Time { return time.Unix(0, unix*int64(time.Millisecond)) }

func TimeToUnix(t time.Time) int64 { return t.UnixNano() / int64(time.Millisecond) }

func ParseStartTime(tStr string) (time.Time, error) {
	if tStr == "" {
		return time.Date(1970, 1, 1, 0, 0, 0, 0, config.C.BJT), nil
	}

	t, err := parseTime(tStr)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

func ParseEndTime(tStr string) (time.Time, error) {
	if tStr == "" {
		return time.Now().In(config.C.BJT), nil
	}

	t, err := parseTime(tStr)
	if err != nil {
		return time.Time{}, err
	}
	return t.Add(24 * time.Hour), nil
}

// parseTime parses a string representation of time in the format "2006-01-02"
// and returns a time.Time value.
func parseTime(s string) (time.Time, error) {
	const timeLayout = "2006-01-02"
	return time.ParseInLocation(timeLayout, s, config.C.BJT)
}
