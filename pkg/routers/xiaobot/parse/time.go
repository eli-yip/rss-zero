package parse

import "time"

// 2024-02-07T14:30:14.000000Z
func (p *ParseService) ParseTime(tStr string) (t time.Time, err error) {
	return time.Parse(time.RFC3339Nano, tStr)
}
