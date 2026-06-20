package migrate

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplyPaidNotice(t *testing.T) {
	const link = "https://zhuanlan.zhihu.com/p/123"
	const notice = "> 本文为付费内容，请点击 [原文链接](https://zhuanlan.zhihu.com/p/123) 查看全文"

	// fresh text gets the linked blockquote prepended
	assert.Equal(t, notice+"\n\nbody", applyPaidNotice("body", link))

	// legacy inline notice is stripped and replaced
	assert.Equal(t, notice+"\n\nbody", applyPaidNotice("**该文章为付费专栏内容**\n\nbody", link))

	// idempotent: an already-backfilled notice is left untouched (keeps its link)
	already := notice + "\n\nbody"
	assert.Equal(t, already, applyPaidNotice(already, "https://example.com/other"))

	// empty body yields just the notice
	assert.Equal(t, notice, applyPaidNotice("", link))
}

func TestPaidNoticeMigrationRegistered(t *testing.T) {
	assert.NoError(t, validateRegistry(registry))

	var found *Migration
	for i := range registry {
		if registry[i].Version == 20260620000000 {
			found = &registry[i]
			break
		}
	}
	if assert.NotNil(t, found, "paid-notice backfill should be registered") {
		assert.Equal(t, "zhihu-paid-notice-backfill", found.Name)
		assert.True(t, found.Auto)
		assert.False(t, found.RequiresPredecessors)
	}
}
