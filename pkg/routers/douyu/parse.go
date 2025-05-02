// logic fromhttps://github.com/DIYgod/RSSHub/blob/master/lib/routes/douyu/room.ts
package douyu

import (
	"encoding/json"
	"fmt"
	"time"
)

type liveInfo struct {
	liveStatus liveStatus
	startTime  time.Time
}

type liveStatus int

const (
	liveStatusOn liveStatus = iota
	liveStatusOff
)

type betardInfo struct {
	ShowStatus int `json:"show_status"`
	VideoLoop  int `json:"videoLoop"`
	ShowTime   int `json:"show_time"`
}

func parseBetardInfo(data []byte) (info *liveInfo, err error) {
	var betard betardInfo
	if err = json.Unmarshal(data, &betard); err != nil {
		return nil, fmt.Errorf("failed to unmarshal betard info: %v", err)
	}

	if betard.ShowStatus != 1 || betard.VideoLoop != 0 {
		return nil, nil
	}

	return &liveInfo{
		liveStatus: liveStatusOn,
		startTime:  time.Unix(int64(betard.ShowTime), 0),
	}, nil
}

type oldApiInfo struct {
	Data struct {
		Online    int    `json:"online"`
		StartTime string `json:"start_time"` // 2025-03-14 19:34:27
	} `json:"data"`
}

func parseOldApi(data []byte) (info *liveInfo, err error) {
	var oldApi oldApiInfo
	if err = json.Unmarshal(data, &oldApi); err != nil {
		return nil, fmt.Errorf("failed to unmarshal old api info: %v", err)
	}

	if oldApi.Data.Online == 0 {
		return nil, nil
	}

	startTime, err := time.Parse("2006-01-02 15:04:05", oldApi.Data.StartTime)
	if err != nil {
		return nil, fmt.Errorf("failed to parse start time: %v", err)
	}

	return &liveInfo{
		liveStatus: liveStatusOn,
		startTime:  startTime,
	}, nil
}
