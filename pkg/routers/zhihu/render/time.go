package render

import (
	"time"

	"github.com/eli-yip/rss-zero/config"
)

func formatTimeForRead(t time.Time) string { return t.In(config.C.BJT).Format("2006年1月2日 15:04") }
