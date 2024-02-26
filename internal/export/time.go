package export

import (
	"time"

	"github.com/eli-yip/rss-zero/config"
)

func ParseStartTime(tStr *string) (time.Time, error) {
	if tStr == nil {
		return time.Time{}, nil
	}

	t, err := parseTime(*tStr)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

func ParseEndTime(tStr *string) (time.Time, error) {
	if tStr == nil {
		return time.Time{}, nil
	}

	t, err := parseTime(*tStr)
	if err != nil {
		return time.Time{}, err
	}
	return t.Add(24 * time.Hour), nil
}

// parseTime parses a string representation of time in the format "2006-01-02"
// and returns a time.Time value.
func parseTime(s string) (time.Time, error) {
	const timeLayout = "2006-01-02"
	return time.ParseInLocation(timeLayout, s, config.BJT)
}
