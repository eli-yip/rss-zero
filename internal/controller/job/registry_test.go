package job

import (
	"testing"

	"github.com/stretchr/testify/assert"

	cronDB "github.com/eli-yip/rss-zero/pkg/cron/db"
)

func TestRegistrySpecLookup(t *testing.T) {
	a := assert.New(t)

	cases := []struct {
		typ       int
		name      string
		jobName   string
		resumable bool
	}{
		{cronDB.TypeZsxq, "zsxq", "zsxq_crawl", true},
		{cronDB.TypeZhihu, "zhihu", "zhihu_crawl", true},
		{cronDB.TypeXiaobot, "xiaobot", "xiaobot_crawl", false},
		{cronDB.TypeGitHub, "github", "github_crawl", false},
	}

	for _, c := range cases {
		byType, ok := SpecByType(c.typ)
		a.True(ok, "SpecByType(%d)", c.typ)
		a.Equal(c.name, byType.Name)

		byName, ok := specByName(c.name)
		a.True(ok, "specByName(%q)", c.name)
		a.Equal(c.typ, byName.Type)

		// round trip is consistent both ways (Build is a func, so compare fields)
		a.Equal(byType.Type, byName.Type)
		a.Equal(byType.Name, byName.Name)

		a.Equal(c.jobName, byType.JobName())
		a.Equal(c.resumable, byType.Resumable)
		a.NotNil(byType.Build)
	}

	_, ok := SpecByType(9999)
	a.False(ok)
	_, ok = specByName("unknown")
	a.False(ok)
}

func TestTypeStrIntRoundTrip(t *testing.T) {
	a := assert.New(t)

	for _, name := range []string{"zsxq", "zhihu", "xiaobot", "github"} {
		i, err := TypeStrToInt(name)
		a.NoError(err)
		got, err := TypeIntToStr(i)
		a.NoError(err)
		a.Equal(name, got)
	}

	_, err := TypeStrToInt("nope")
	a.Error(err)
	_, err = TypeIntToStr(9999)
	a.Error(err)
}
