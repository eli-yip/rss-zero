package weibochat

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type memoryUserStore struct{ user *User }

func (s *memoryUserStore) GetUser(context.Context, string) (*User, error) {
	if s.user == nil {
		return nil, nil
	}
	copy := *s.user
	return &copy, nil
}
func (s *memoryUserStore) SaveUser(_ context.Context, user *User) error {
	copy := *user
	s.user = &copy
	return nil
}

func TestAvatarSignatureRotation(t *testing.T) {
	data := testPNG(t)
	downloads := 0
	client := NewClient(&http.Client{Transport: transportFunc(func(*http.Request) (*http.Response, error) {
		downloads++
		return &http.Response{StatusCode: 200, ContentLength: int64(len(data)), Body: io.NopCloser(bytes.NewReader(data))}, nil
	})}, func(context.Context) (string, error) { return "test", nil })
	store, files := &memoryUserStore{}, &memoryObjects{}
	archiver := NewUserArchiver(client, store, files)
	first := "https://tvax1.sinaimg.cn/avatar/first.jpg?KID=1&Expires=100&ssig=old"
	rotated := "https://tvax1.sinaimg.cn/avatar/first.jpg?KID=1&Expires=200&ssig=new"
	changed := "https://tvax1.sinaimg.cn/avatar/changed.jpg?KID=1&Expires=200&ssig=new"
	for i, source := range []string{first, rotated, changed} {
		err := archiver.SyncUsers(context.Background(), []Message{{FetchedAt: time.Now(), Sender: &User{UID: "42", AvatarSourceURL: source}}})
		require.NoError(t, err)
		require.Equal(t, source, store.user.AvatarSourceURL)
		want := 1
		if i == 2 {
			want = 2
		}
		require.Equal(t, want, downloads)
		require.Len(t, files.keys, want)
	}
	require.NotEqual(t, files.keys[0], files.keys[1])
	require.Equal(t, avatarIdentity(first), avatarIdentity(rotated))
	require.NotEqual(t, avatarIdentity(first), avatarIdentity("https://tvax1.sinaimg.cn/avatar/first.jpg?KID=2&Expires=100&ssig=old"))
	require.NotEqual(t, avatarIdentity("https://other.example/avatar?Expires=100"), avatarIdentity("https://other.example/avatar?Expires=200"))
}
