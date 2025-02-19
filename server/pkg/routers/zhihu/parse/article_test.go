package parse

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArticleList(t *testing.T) {
	assert := assert.New(t)

	type testCase struct {
		input  string
		expect int
	}

	type jsonStruct struct {
		ID any `json:"id"`
	}

	testCases := []testCase{
		{`{"id": 100}`, 100},
		{`{"id": "100"}`, 100},
	}

	for _, tc := range testCases {
		js := jsonStruct{}
		err := json.Unmarshal([]byte(tc.input), &js)
		if err != nil {
			t.Fatalf("failed to unmarshal json: %v", err)
		}

		if f, ok := js.ID.(float64); ok {
			assert.Equal(tc.expect, int(f))
		} else if s, ok := js.ID.(string); ok {
			i, err := strconv.Atoi(s)
			assert.Nil(err)
			assert.Equal(tc.expect, i)
		} else {
			t.Fatalf("failed to convert article id from any to int, data: %s", tc.input)
		}
	}
}
