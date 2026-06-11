package cookie

import (
	"errors"
	"testing"
	"time"

	"go.uber.org/zap"
)

type fakeStore struct {
	vals    map[int]string
	deleted []int
}

func newFakeStore() *fakeStore { return &fakeStore{vals: map[int]string{}} }

func (f *fakeStore) Set(t int, v string, _ time.Duration) error { f.vals[t] = v; return nil }
func (f *fakeStore) Get(t int) (string, error) {
	if v, ok := f.vals[t]; ok {
		return v, nil
	}
	return "", ErrKeyNotExist
}
func (f *fakeStore) GetCookieTypes() ([]int, error) { return nil, nil }
func (f *fakeStore) Check(t int) error {
	if _, ok := f.vals[t]; ok {
		return nil
	}
	return ErrKeyNotExist
}
func (f *fakeStore) CheckTTL(t int, _ time.Duration) error { return f.Check(t) }
func (f *fakeStore) GetTTL(int) (time.Duration, error)     { return time.Hour, nil }
func (f *fakeStore) Del(t int) error {
	delete(f.vals, t)
	f.deleted = append(f.deleted, t)
	return nil
}

type fakeNotifier struct {
	count int
	last  string
}

func (n *fakeNotifier) Notify(title, _ string) error { n.count++; n.last = title; return nil }

func TestBundleAllPresent(t *testing.T) {
	store := newFakeStore()
	store.vals[CookieTypeZhihuDC0] = "dc0"
	store.vals[CookieTypeZhihuZC0] = "zc0"
	store.vals[CookieTypeZhihuZSECK] = "zse"
	n := &fakeNotifier{}

	m, err := Bundle(store, "zhihu", n, zap.NewNop())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m["d_c0"] != "dc0" || m["z_c0"] != "zc0" || m["__zse_ck"] != "zse" {
		t.Fatalf("unexpected map: %#v", m)
	}
	if n.count != 0 {
		t.Fatalf("did not expect a notification, got %d", n.count)
	}
}

func TestBundleMissingNotifiesAndStops(t *testing.T) {
	store := newFakeStore()
	store.vals[CookieTypeZhihuDC0] = "dc0"
	store.vals[CookieTypeZhihuZC0] = "zc0"
	// __zse_ck deliberately absent
	n := &fakeNotifier{}

	_, err := Bundle(store, "zhihu", n, zap.NewNop())
	if !errors.Is(err, ErrCookieMissing) {
		t.Fatalf("expected ErrCookieMissing, got %v", err)
	}
	if n.count != 1 {
		t.Fatalf("expected exactly one notification, got %d", n.count)
	}
}

func TestBundleEmptyValueTreatedAsMissing(t *testing.T) {
	store := newFakeStore()
	store.vals[CookieTypeZsxqAccessToken] = ""
	n := &fakeNotifier{}

	_, err := Bundle(store, "zsxq", n, zap.NewNop())
	if !errors.Is(err, ErrCookieMissing) {
		t.Fatalf("expected ErrCookieMissing for empty value, got %v", err)
	}
	if n.count != 1 {
		t.Fatalf("expected one notification, got %d", n.count)
	}
}

func TestInvalidateDeletesAndNotifies(t *testing.T) {
	store := newFakeStore()
	store.vals[CookieTypeZsxqAccessToken] = "token"
	n := &fakeNotifier{}

	Invalidate(store, CookieTypeZsxqAccessToken, n, zap.NewNop())

	if _, ok := store.vals[CookieTypeZsxqAccessToken]; ok {
		t.Fatal("expected cookie to be deleted")
	}
	if len(store.deleted) != 1 || store.deleted[0] != CookieTypeZsxqAccessToken {
		t.Fatalf("expected Del called once for zsxq, got %#v", store.deleted)
	}
	if n.count != 1 {
		t.Fatalf("expected one notification, got %d", n.count)
	}
}
