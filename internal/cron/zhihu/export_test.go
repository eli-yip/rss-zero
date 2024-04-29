package cron

import (
	"testing"
	"time"
)

func TestGetStartDateAndEndDate(t *testing.T) {
	now := time.Date(2021, 6, 1, 0, 0, 0, 0, time.UTC)
	startDate, endDate := getStartDateEndDate(now)
	if startDate != "2021-06-01" || endDate != "2021-06-30" {
		t.Errorf("getStartDateEndDate(now) = (%s, %s); want (2021-06-01, 2021-06-30)", startDate, endDate)
	}
}
