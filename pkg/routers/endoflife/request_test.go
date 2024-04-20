package endoflife

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetReleaseCycles(t *testing.T) {
	assert := assert.New(t)
	cycles, err := GetReleaseCycles("mattermost")
	assert.Nil(err)
	t.Logf("%+v", cycles)
}
