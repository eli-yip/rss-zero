package main

import (
	"context"
	"errors"
	"testing"

	"github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/eli-yip/rss-zero/pkg/cookie"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestRunWeiboGroups(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want []string
	}{
		{name: "all groups", want: []string{"1", "2"}},
		{name: "group failure continues", err: errors.New("group unavailable"), want: []string{"1", "2"}},
		{name: "missing credential stops", err: cookie.ErrCookieMissing, want: []string{"1"}},
		{name: "rejected credential stops", err: &weibochat.APIError{Code: "21301"}, want: []string{"1"}},
		{name: "other API error continues", err: &weibochat.APIError{Code: "10012"}, want: []string{"1", "2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var called []string
			runWeiboGroups([]string{"1", "2"}, func(_ context.Context, group string) error {
				called = append(called, group)
				return tc.err
			}, zap.NewNop())
			require.Equal(t, tc.want, called)
		})
	}
}
