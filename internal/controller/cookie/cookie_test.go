package controller

import (
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/pkg/cookie"
)

type fakeStore struct{ vals map[int]string }

func newFakeStore() *fakeStore { return &fakeStore{vals: map[int]string{}} }

func (f *fakeStore) Set(t int, v string, _ time.Duration) error { f.vals[t] = v; return nil }
func (f *fakeStore) Get(t int) (string, error) {
	if v, ok := f.vals[t]; ok {
		return v, nil
	}
	return "", cookie.ErrKeyNotExist
}
func (f *fakeStore) GetCookieTypes() ([]int, error) { return nil, nil }
func (f *fakeStore) Check(int) error                { return nil }
func (f *fakeStore) CheckTTL(int, time.Duration) error {
	return nil
}
func (f *fakeStore) GetTTL(int) (time.Duration, error) { return time.Hour, nil }
func (f *fakeStore) Del(t int) error                   { delete(f.vals, t); return nil }

func TestStore(t *testing.T) {
	tenDays := float64(time.Now().Add(10 * 24 * time.Hour).Unix())
	past := float64(time.Now().Add(-time.Hour).Unix())

	tests := []struct {
		name       string
		in         InCookie
		wantStored bool
		wantReason string
		wantValue  string // expected stored value for CookieTypeZhihuDC0, if stored
	}{
		{
			name:       "not registered",
			in:         InCookie{Name: "_ga", Value: "x", ExpirationDate: new(tenDays)},
			wantReason: "not registered",
		},
		{
			name:       "empty value",
			in:         InCookie{Name: "d_c0", Value: "", ExpirationDate: new(tenDays)},
			wantReason: "empty value",
		},
		{
			name:       "already expired",
			in:         InCookie{Name: "d_c0", Value: "d_c0=abc", ExpirationDate: new(past)},
			wantReason: "already expired",
		},
		{
			name:       "stored strips name prefix",
			in:         InCookie{Name: "d_c0", Value: "d_c0=abc", ExpirationDate: new(tenDays)},
			wantStored: true,
			wantValue:  "abc",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store := newFakeStore()
			h := NewController(store)
			res := h.store(tc.in, zap.NewNop())

			if res.Stored != tc.wantStored {
				t.Fatalf("Stored = %v, want %v (reason %q)", res.Stored, tc.wantStored, res.Reason)
			}
			if tc.wantReason != "" && res.Reason != tc.wantReason {
				t.Fatalf("Reason = %q, want %q", res.Reason, tc.wantReason)
			}
			if tc.wantStored && store.vals[cookie.CookieTypeZhihuDC0] != tc.wantValue {
				t.Fatalf("stored value = %q, want %q", store.vals[cookie.CookieTypeZhihuDC0], tc.wantValue)
			}
		})
	}
}
