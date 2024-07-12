package cron

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterGroupIDs(t *testing.T) {
	assert := assert.New(t)

	type Case struct {
		include []string
		exclude []string
		all     []int
		expect  []int
	}

	cases := []Case{
		{
			include: []string{},
			exclude: []string{},
			all:     []int{28855218411241},
			expect:  []int{28855218411241},
		},
		{
			include: []string{""},
			exclude: []string{""},
			all:     []int{28855218411241},
			expect:  []int{28855218411241},
		},
	}

	for _, c := range cases {
		results, err := FilterGroupIDs(c.include, c.exclude, c.all)
		assert.Nil(err)
		assert.Equal(c.expect, results)
	}
}
