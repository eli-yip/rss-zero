package md

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type testCase struct {
	input  string
	expect string
}

func TestH1(t *testing.T) {
	testCases := []testCase{
		{"Hello", "# Hello"},
	}

	assert := assert.New(t)
	for _, tc := range testCases {
		assert.Equal(H1(tc.input), tc.expect)
	}
}

func TestH2(t *testing.T) {
	testCases := []testCase{
		{"Hello", "## Hello"},
	}

	assert := assert.New(t)
	for _, tc := range testCases {
		assert.Equal(H2(tc.input), tc.expect)
	}
}

func TestH3(t *testing.T) {
	testCases := []testCase{
		{"Hello", "### Hello"},
	}

	assert := assert.New(t)
	for _, tc := range testCases {
		assert.Equal(H3(tc.input), tc.expect)
	}
}

func TestQuote(t *testing.T) {
	testCases := []testCase{
		{"Hello", "> Hello"},
		{"Hello\nWorld", "> Hello\n> World"},
		{"Hello\nWorld\n\n", "> Hello\n> World"},
		{"Hello\n\nWorld", "> Hello\n> \n> World"},
		{"", ""},
	}

	assert := assert.New(t)
	for _, tc := range testCases {
		assert.Equal(Quote(tc.input), tc.expect)
	}
}

func TestBold(t *testing.T) {
	testCases := []testCase{
		{"Hello", "**Hello**"},
	}

	assert := assert.New(t)
	for _, tc := range testCases {
		assert.Equal(Bold(tc.input), tc.expect)
	}
}
