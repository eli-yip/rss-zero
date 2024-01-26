package parse

import "testing"

func TestStrToInt(t *testing.T) {
	var tests = []string{
		"abc",
		"https://zhihu.com",
	}

	for _, test := range tests {
		t.Logf("strToInt(%s) = %d", test, urlToID(test))
	}
}

func TestReplaceImageLinks(t *testing.T) {
	type cases struct {
		content string
		name    string
		from    string
		to      string
		result  string
	}

	casesList := []cases{
		{
			content: `![image](http`,
			name:    `image`,
			from:    `http`,
			to:      `https`,
			result:  `![image](http`,
		},
		{
			content: `![image](http://abc.com)`,
			name:    `image`,
			from:    `http://abc.com`,
			to:      `https://abc.com`,
			result:  `![image](https://abc.com)`,
		},
	}

	for _, c := range casesList {
		result := replaceImageLinks(c.content, c.name, c.from, c.to)
		if result != c.result {
			t.Errorf("expected %s, got %s", c.result, result)
		}
	}
}
