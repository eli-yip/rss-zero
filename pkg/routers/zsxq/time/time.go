package parse

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

const ZsxqTimeLayout = "2006-01-02T15:04:05.999999-0700"

// EncodeTimeForQuery encode string to Zhishixingqiu time string,
// which will be used in request url params.
// It takes decoded time string as input,
// and returns encoded time string.
func EncodeTimeForQuery(decTimeStr string) string {
	parts := strings.SplitN(decTimeStr, "T", 2)
	if len(parts) == 2 {
		datePart := parts[0]
		timePart := parts[1]

		// Parse the time and subtract 1 millisecond
		date, err := time.Parse(ZsxqTimeLayout, decTimeStr)
		if err == nil {
			// successfully parsed
			date = date.Add(time.Duration(-1) * time.Millisecond)

			// Format date and time ensuring fixed width
			year, month, day := date.Date()
			hour, min, sec := date.Clock()
			millis := date.Nanosecond() / 1e6
			tzOffset := date.Format("-0700")

			decTimeStr = fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d.%03d%s",
				year, month, day, hour, min, sec, millis, tzOffset)
			parts = strings.SplitN(decTimeStr, "T", 2)
			if len(parts) == 2 {
				timePart = parts[1]
			}
		}

		encodedTime := url.QueryEscape(timePart)
		return fmt.Sprintf("%sT%s", datePart, encodedTime)
	}
	return decTimeStr
}

// DecodeStringToTime parse Zhishixingqiu time string to time.Time.
// e.g.: 2020-01-01T00:00:00.000000+0800 -> 2020-01-01 00:00:00 +0800 CST
func DecodeStringToTime(decTimeStr string) (result time.Time, err error) {
	result, err = time.Parse(ZsxqTimeLayout, decTimeStr)
	if err != nil {
		return time.Time{}, err
	}

	return result, nil
}

// EncodeTimeToString encode time.Time to Zhishixingqiu time string.
// then the string will be processed by EncodeTime function.
func EncodeTimeToString(t time.Time) string {
	return t.Format(ZsxqTimeLayout)
}

// FormatTimeForRead format Zhishixingqiu time string to "2006年1月2日".
func FormatTimeForRead(timestr string) (string, error) {
	date, err := time.Parse(ZsxqTimeLayout, timestr)
	if err != nil {
		return "", err
	}

	// Convert to local timezone (if it's not already)
	date = date.Local()

	outputFormat := "2006年1月2日"
	return date.Format(outputFormat), nil
}
