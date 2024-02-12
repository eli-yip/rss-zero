package encrypt

import (
	"testing"
	"time"
)

type testCase struct {
	t         time.Time
	u         string
	timestamp string
	want      string
}

func TestSign(t *testing.T) {
	location, _ := time.LoadLocation("Asia/Shanghai")
	cases := []testCase{
		{
			t:         time.Date(2024, 2, 12, 8, 25, 48, 0, location),
			u:         "https://api.xiaobot.net/paper/smalltalk2023?refer_channel=",
			timestamp: "1707697548",
			want:      "30e555c3f3cc524ec26a7808953a5648",
		},
		{
			t:         time.Date(2024, 2, 12, 8, 50, 11, 0, location),
			u:         "https://api.xiaobot.net/paper/subscribed",
			timestamp: "1707699011",
			want:      "8b5b5ee82032def0c4edd209ef897190",
		},
		{
			t:         time.Date(2024, 2, 12, 9, 9, 21, 0, location),
			u:         "https://api.xiaobot.net/paper/smalltalk2023/post?limit=20&offset=0&tag_name=&keyword=&order_by=created_at+undefined",
			timestamp: "1707700161",
			want:      "a5165738f4293f345be1c2ba3b1da518",
		},
	}

	for _, c := range cases {
		timestamp, sign, err := Sign(c.t, c.u)
		if err != nil {
			t.Error(err)
		}
		if timestamp != c.timestamp || sign != c.want {
			t.Errorf("Sign(%v) == (%v, %v), want (%v, %v)", c.t, timestamp, sign, c.timestamp, c.want)
		}
	}
}

func TestParseParams(t *testing.T) {
	cases := []struct {
		u    string
		want string
	}{
		{
			u:    "https://api.xiaobot.net/paper/smalltalk2023/post?limit=20&offset=0&tag_name=&keyword=&order_by=created_at+undefined",
			want: "keyword=&limit=20&offset=0&order_by=created_at undefined&tag_name=",
		},
	}

	for _, c := range cases {
		got, err := parseQuery(c.u)
		if err != nil {
			t.Error(err)
		}
		if got != c.want {
			t.Errorf("parseQuery(%v) == %v, want %v", c.u, got, c.want)
		}
	}
}
