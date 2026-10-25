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

func TestListSendersAndGetUsers(t *testing.T) {
	store, _ := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	_, err := store.SaveMessages(ctx, []Message{
		{GroupID: "1", MessageID: "1", SenderID: "9", Raw: []byte(`{}`), FetchedAt: now},
		{GroupID: "1", MessageID: "2", SenderID: "9", Raw: []byte(`{}`), FetchedAt: now},
		{GroupID: "1", MessageID: "3", SenderID: "10", Raw: []byte(`{}`), FetchedAt: now},
		{GroupID: "1", MessageID: "4", Raw: []byte(`{}`), FetchedAt: now},
		{GroupID: "2", MessageID: "5", SenderID: "20", Raw: []byte(`{}`), FetchedAt: now},
	})
	require.NoError(t, err)
	require.NoError(t, store.SaveUser(ctx, &User{UID: "9", ScreenName: "old", ProfileObservedAt: now}))
	require.NoError(t, store.SaveUser(ctx, &User{UID: "9", ScreenName: "latest", AvatarObjectKey: "avatar.jpg", ProfileObservedAt: now.Add(time.Second)}))
	require.NoError(t, store.SaveUser(ctx, &User{UID: "20", ScreenName: "other group", ProfileObservedAt: now}))
	senders, err := store.ListSenders(ctx, "1")
	require.NoError(t, err)
	require.Len(t, senders, 2)
	require.Equal(t, "9", senders[0].UID)
	require.Equal(t, "latest", senders[0].ScreenName)
	require.Equal(t, "avatar.jpg", senders[0].AvatarObjectKey)
	require.Equal(t, User{UID: "10"}, senders[1])
	users, err := store.GetUsers(ctx, []string{"9", "10"})
	require.NoError(t, err)
	require.Len(t, users, 1)
	require.Equal(t, "9", users[0].UID)
	empty, err := store.GetUsers(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, empty)
}
