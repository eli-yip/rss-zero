package job

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRegistrySpecByKind(t *testing.T) {
	cases := []struct {
		kind      string
		jobName   string
		resumable bool
	}{
		{kind: "zsxq", jobName: "zsxq_crawl", resumable: true},
		{kind: "zhihu", jobName: "zhihu_crawl", resumable: true},
		{kind: "xiaobot", jobName: "xiaobot_crawl", resumable: false},
		{kind: "github", jobName: "github_crawl", resumable: false},
	}

	for _, tc := range cases {
		t.Run(tc.kind, func(t *testing.T) {
			spec, ok := SpecByKind(tc.kind)
			assert.True(t, ok)
			assert.Equal(t, tc.kind, spec.Kind)
			assert.Equal(t, tc.jobName, spec.JobName())
			assert.Equal(t, tc.resumable, spec.Resumable)
			assert.NotNil(t, spec.Build)
		})
	}
}

func TestRegistrySpecByKindUnknown(t *testing.T) {
	_, ok := SpecByKind("unknown")
	assert.False(t, ok)
}
