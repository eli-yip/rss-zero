package encrypt

import (
	"testing"
	"time"
)

type testCase struct {
	t         time.Time
	timestamp string
	want      string
}

func TestSign(t *testing.T) {
	location, _ := time.LoadLocation("Asia/Shanghai")
	cases := []testCase{
		{
			t:         time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			timestamp: "1577836800",
			want:      "4d7308adfc6afde6e15e686b40159f03",
		},
		{
			t:         time.Date(2020, 1, 1, 0, 0, 0, 0, location),
			timestamp: "1577808000",
			want:      "bc444548937130f3495011015bece1f8",
		},
	}

	for _, c := range cases {
		timestamp, sign := Sign(c.t)
		if timestamp != c.timestamp || sign != c.want {
			t.Errorf("Sign(%v) == (%v, %v), want (%v, %v)", c.t, timestamp, sign, c.timestamp, c.want)
		}
	}
}
