package db

import "time"

type DataBaseIface interface {
	GetZsxqGroupIDs() ([]int, error)
	GetLatestTopicTime(groupID int) (time.Time, error)
	SaveLatestTime(groupID int, latestTime time.Time) error
}
