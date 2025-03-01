package macked

import (
	"html"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsSubscribed(t *testing.T) {
	type testCase struct {
		title    string
		expected bool
	}

	names := []string{
		"MacWhisper",
		"Parallels Desktop",
		"Beyond Compare",
		"Text Workflow",
	}

	cases := []testCase{
		{`MacWhisper 11.10 破解版 &#8211; macOS好用的转录软件`, true},
		{`Parallels Desktop 20 20.2.2-55879(修复盗版弹窗/激活工具6.8.0) 破解版 &#8211; PD虚拟机破解工具/激活补丁/破解补丁`, true},
		{`Color Wheel 8.5 破解版 &#8211; macOS数字色轮工具`, false},
	}

	assert := assert.New(t)

	for _, c := range cases {
		assert.Equal(c.expected, isSubscribed(html.UnescapeString(c.title), names))
	}
}
