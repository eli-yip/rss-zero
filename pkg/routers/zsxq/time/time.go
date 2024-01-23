package time

import (
	"fmt"
	"net/url"
	"time"
)

// EncodeTimeForQuery encode time.Time for zsxq query time param.
//
// e.g.: 2023-12-17T15%3A44%3A11.891%2B0800
func EncodeTimeForQuery(t time.Time) string {
	// subtract 1 millisecond to ensure query offset
	t = t.Add(time.Duration(-1) * time.Millisecond)

	// Format date and time to ensure str has a fixed width
	year, month, day := t.Date()
	hour, min, sec := t.Clock()
	millis := t.Nanosecond() / 1e6
	tzOffset := t.Format("-0700")

	datePart := fmt.Sprintf("%04d-%02d-%02d", year, month, day)

	timePart := fmt.Sprintf("%02d:%02d:%02d.%03d%s",
		hour, min, sec, millis, tzOffset)
	encodedTime := url.QueryEscape(timePart)

	return fmt.Sprintf("%sT%s", datePart, encodedTime)
}

// DecodeZsxqAPITime parse zsxq time string to time.Time.
//
// e.g.: "2024-01-22T12:19:44.405+0800" -> time.Date(2024, 1, 22, 12, 19, 44, 405000000, location),
func DecodeZsxqAPITime(ts string) (result time.Time, err error) {
	const zsxqTimeLayout = "2006-01-02T15:04:05.000-0700"

	result, err = time.Parse(zsxqTimeLayout, ts)
	if err != nil {
		return time.Time{}, err
	}
	return result, nil
}

// FmtForRead format time.Time to a time string like "2006年1月2日".
func FmtForRead(t time.Time) (string, error) {
	const ZsxqTimeLayoutForRead = "2006年1月2日"

	// set location to China
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return "", err
	}
	t = t.In(location)

	return t.Format(ZsxqTimeLayoutForRead), nil
}
