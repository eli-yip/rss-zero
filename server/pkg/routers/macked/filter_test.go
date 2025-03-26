package macked

import (
	"html"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSubscribed(t *testing.T) {
	type testCase struct {
		title    string
		expected int
	}

	names := []string{
		"MacWhisper",
		"Parallels Desktop",
		"Beyond Compare",
		"Text Workflow",
	}

	cases := []testCase{
		{`MacWhisper 11.10 破解版 &#8211; macOS好用的转录软件`, 0},
		{`Parallels Desktop 20 20.2.2-55879(修复盗版弹窗/激活工具6.8.0) 破解版 &#8211; PD虚拟机破解工具/激活补丁/破解补丁`, 1},
		{`Color Wheel 8.5 破解版 &#8211; macOS数字色轮工具`, -1},
	}

	assert := assert.New(t)

	for tc := range slices.Values(cases) {
		assert.Equal(tc.expected, appIndex(html.UnescapeString(tc.title), names))
	}
}
