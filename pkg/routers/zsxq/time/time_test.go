package time

import (
	"testing"
	"time"

	"github.com/eli-yip/rss-zero/config"
	"github.com/stretchr/testify/assert"
)

type testCase struct {
	input time.Time
	want  string
}

func TestEncodeTimeForQuery(t *testing.T) {
	testCases := []testCase{
		{
			// equals to zsxq api time str "2023-09-14T21:51:50.943+0800"
			// Use 943000000 to ensure nanosecond is 9 digits,
			// 943 is millisecond, 943000000 is nanosecond
			time.Date(2023, 9, 14, 21, 51, 50, 943000000, config.BJT),
			"2023-09-14T21%3A51%3A50.942%2B0800",
		},
		{
			// equals to zsxq api time str "2023-08-30T19:59:22.593+0800"
			time.Date(2023, 8, 30, 19, 59, 22, 593000000, config.BJT),
			"2023-08-30T19%3A59%3A22.592%2B0800",
		},
	}

	assert := assert.New(t)
	for _, tc := range testCases {
		got := EncodeTimeForQuery(tc.input)
		assert.Equal(tc.want, got)
	}
}

func TestDecodeZsxqAPITime(t *testing.T) {
	type testCase struct {
		input string
		want  time.Time
	}

	testCases := []testCase{
		{
			"2024-01-22T14:56:02.297+0800",
			time.Date(2024, 1, 22, 14, 56, 02, 297000000, config.BJT),
		},
		{
			// equals to zsxq api time str "2023-08-30T19:59:22.593+0800"
			"2024-01-22T12:19:44.405+0800",
			time.Date(2024, 1, 22, 12, 19, 44, 405000000, config.BJT),
		},
	}

	assert := assert.New(t)
	for _, tc := range testCases {
		got, err := DecodeZsxqAPITime(tc.input)
		assert.Nil(err)
		assert.Equal(tc.want, got)
	}
}

func TestFmtForRead(t *testing.T) {
	testCases := []testCase{
		{
			// equals to zsxq api time str "2023-08-30T19:59:22.593+0800"
			time.Date(2023, 8, 30, 19, 59, 22, 593000000, config.BJT),
			"2023年8月30日",
		},
		{
			// equals to zsxq api time str "2023-08-30T19:59:22.593+0800"
			time.Date(2023, 1, 1, 22, 0, 0, 0, time.UTC),
			"2023年1月2日",
		},
	}

	assert := assert.New(t)
	for _, tc := range testCases {
		got, err := FmtForRead(tc.input)
		assert.Nil(err)
		assert.Equal(tc.want, got)
	}
}
