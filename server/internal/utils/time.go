package utils

import "time"

func UnixToTime(unix int64) time.Time { return time.Unix(0, unix*int64(time.Millisecond)) }

func TimeToUnix(t time.Time) int64 { return t.UnixNano() / int64(time.Millisecond) }
