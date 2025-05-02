package endoflife

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseCycles(t *testing.T) {
	cycles := []cycle{
		{
			Cycle:             "1.0.0",
			ReleaseDate:       "2021-01-01",
			Eol:               "2021-01-01",
			Latest:            "1.0.0",
			LatestReleaseDate: "2021-01-01",
			Lts:               false,
		},
		{
			Cycle:             "2.0.0",
			ReleaseDate:       "2021-01-01",
			Eol:               "2021-01-01",
			Latest:            "2.0.0",
			LatestReleaseDate: "2021-01-01",
			Lts:               false,
		},
		{
			Cycle:             "3.0.0",
			ReleaseDate:       "2021-01-01",
			Eol:               "2021-01-01",
			Latest:            "3.0.0",
			LatestReleaseDate: "2021-01-01",
			Lts:               false,
		},
	}

	expect := []versionInfo{
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
			releaseDate: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		{
			version: version{
				major: 1,
				minor: 0,
				Patch: 0,
			},
			releaseDate: time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	assert := assert.New(t)
	versionInfoList, err := ParseCycles(cycles)
	assert.Nil(err)
	assert.Equal(expect, versionInfoList)
}
