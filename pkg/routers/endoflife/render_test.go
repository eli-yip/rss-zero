package endoflife

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRenderRSS(t *testing.T) {
	renderService := NewRSSRenderService()
	assert := assert.New(t)
	type testCase struct {
		product     string
		versionList []versionInfo
	}
	testCases := []testCase{{
		product: "mattermost",
		versionList: []versionInfo{
			{
				version: version{
					major: 3,
					minor: 0,
					Patch: 0,
				},
				releaseDate: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
			},
			{
				version: version{
					major: 2,
					minor: 0,
					Patch: 0,
				},
				releaseDate: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}}

	for _, tc := range testCases {
		rss, err := renderService.RenderRSS(tc.product, tc.versionList)
		assert.Nil(err)
		assert.NotEmpty(rss)
		t.Log(rss)
	}
}
