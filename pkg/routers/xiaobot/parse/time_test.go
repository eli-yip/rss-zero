package parse

import (
	"testing"
	"time"
)

type timeTest struct {
	timeStr string
	t       time.Time
}

func TestTimeParse(t *testing.T) {
	timeTests := []timeTest{
		{
			timeStr: "2024-02-07T14:30:14.000000Z",
			t:       time.Date(2024, 2, 7, 14, 30, 14, 0, time.UTC),
		},
		{
			timeStr: "2024-02-01T13:59:12.000000Z",
			t:       time.Date(2024, 2, 1, 13, 59, 12, 0, time.UTC),
		},
	}

	parseService := NewParseService(nil, nil, nil, nil)

	for _, tt := range timeTests {
		tt := tt
		t.Run(tt.timeStr, func(t *testing.T) {
			t.Parallel()
			tm, err := parseService.ParseTime(tt.timeStr)
			if err != nil {
				t.Errorf("parseTime(%s) got error: %v", tt.timeStr, err)
			}
			if !tm.Equal(tt.t) {
				t.Errorf("parseTime(%s) = %v, want %v", tt.timeStr, tm, tt.t)
			}
		})
	}
}
