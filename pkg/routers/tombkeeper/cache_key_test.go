package tombkeeper

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	redisx "github.com/eli-yip/rss-zero/internal/redis"
)

type recordingRedis struct{ setKey string }

func (r *recordingRedis) Set(key string, _ any, _ time.Duration) error { r.setKey = key; return nil }
func (*recordingRedis) Get(string) (string, error)                     { return "", errors.New("unused") }
func (*recordingRedis) Del(string) error                               { return nil }
func (*recordingRedis) TTL(string) (time.Duration, error)              { return 0, nil }

func TestRenderAndCacheRSSUsesStructuredContentKey(t *testing.T) {
	cache := &recordingRedis{}
	require.NoError(t, renderAndCacheRSS(cache, newFakeDB(), testLogger()))
	want := "v2:" + redisx.RssTombkeeperTimelinePath
	require.Equal(t, want, cache.setKey)
}
