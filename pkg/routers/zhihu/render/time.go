package render

import "time"

func formatTimeForRead(t time.Time) string {
	location, _ := time.LoadLocation("Asia/Shanghai")
	t = t.In(location)
	return t.Format("2006年1月2日 15:04")
}
