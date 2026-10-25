package weibochat

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestParseSearchQuery(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  []string
	}{
		{"  \t\n", []string{}},
		{"数据库" + "\u3000" + "索引\nhello", []string{"数据库", "索引", "hello"}},
		{`数据库 "hello  world"`, []string{"数据库", "hello  world"}},
		{`a\"b c\\d C:\tmp 100% _ '单引号' “中文引号”`, []string{`a"b`, `c\d`, `C:\tmp`, "100%", "_", "'单引号'", "“中文引号”"}},
	} {
		t.Run(tc.input, func(t *testing.T) {
			q, err := ParseSearchQuery(tc.input)
			require.NoError(t, err)
			require.Equal(t, tc.want, q.Terms)
		})
	}
	for _, input := range []string{`"`, `"abc`, `""`, `foo""`, strings.Repeat("中", 257)} {
		_, err := ParseSearchQuery(input)
		require.Error(t, err, input)
	}
	_, err := ParseSearchQuery(strings.Repeat("中", 256))
	require.NoError(t, err)
}

func TestSearchMatchesUseOriginalCodePoints(t *testing.T) {
	q, err := ParseSearchQuery(`数据库 库索 HELLO`)
	require.NoError(t, err)
	text := "🙂数据库索引 hello HELLO"
	require.Equal(t, []TextMatch{{Start: 1, End: 5}, {Start: 7, End: 12}, {Start: 13, End: 18}}, q.FindMatches(text))
	q, err = ParseSearchQuery("aa")
	require.NoError(t, err)
	require.Equal(t, []TextMatch{{Start: 0, End: 4}}, q.FindMatches("aaaa"))
	require.Equal(t, []TextMatch{}, q.FindMatches("数据库"))
	require.Equal(t, []TextMatch{}, (SearchQuery{}).FindMatches(text))
	q, err = ParseSearchQuery("é")
	require.NoError(t, err)
	require.Equal(t, []TextMatch{{Start: 1, End: 2}}, q.FindMatches("Éé"))
}
