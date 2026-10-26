package weibochat

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseConfirmedGroupEvents(t *testing.T) {
	for _, tc := range []struct {
		name  string
		raw   string
		kind  EventType
		actor *EventActor
	}{
		{"joined", `"type":322,"template":"{{name.DATA}} 加入了群","data":{"name":{"value":"新人","scheme":"sinaweibo://userinfo?uid=23"}}`, EventMemberJoined, &EventActor{UID: "23", ScreenName: "新人"}},
		{"recalled", `"type":344,"content":"观察者 撤回了一条消息"`, EventMessageRecalled, &EventActor{UID: "42", ScreenName: "观察者"}},
		{"published", `"type":100,"content":"我发布了新微博，快来围观吧！","url_objects":[{"url_ori":"https://weibo.com/42/abc"}]`, EventWeiboPublished, &EventActor{UID: "42", ScreenName: "观察者"}},
		{"unknown template", `"type":322,"template":"{{name.DATA}} 离开了群"`, EventNone, nil},
		{"ordinary recalled text", `"type":321,"content":"观察者 撤回了一条消息"`, EventNone, nil},
		{"unconfirmed recalled name", `"type":344,"content":"他人 撤回了一条消息"`, EventNone, nil},
		{"ordinary notice text", `"type":321,"template":"涉及资金问题请务必提高警惕，谨防诈骗。{{cases.DATA}}"`, EventNone, nil},
		{"unknown data shape", `"type":999,"data":true,"template":123`, EventNone, nil},
		{"unknown type", `"type":999,"template":"{{name.DATA}} 加入了群"`, EventNone, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw := []byte(fmt.Sprintf(`{"gid":1,"id":2,"from_uid":42,"from_user":{"screen_name":"观察者"},"media_type":0,"sub_type":101,%s}`, tc.raw))
			message, err := ParseMessage(raw)
			require.NoError(t, err)
			require.Empty(t, message.ParseError)
			require.Equal(t, tc.kind, message.EventType)
			require.Equal(t, tc.actor, message.Content.EventActor)
			require.JSONEq(t, string(raw), string(message.Raw))
			if tc.kind == EventNone {
				require.Equal(t, "42", message.SenderID)
				require.NotNil(t, message.Sender)
			} else {
				require.Empty(t, message.SenderID)
				require.Nil(t, message.Sender)
			}
			if tc.kind == EventWeiboPublished {
				require.Equal(t, []Link{{OriginalURL: "https://weibo.com/42/abc"}}, message.Content.Links)
			}
		})
	}
}

func TestParseGroupNoticeWhitelist(t *testing.T) {
	for _, template := range []string{
		"涉及资金问题请务必提高警惕，谨防诈骗。{{cases.DATA}}",
		"恭喜{{nick.DATA}}今日获得“{{title.DATA}}”标识",
		"恭喜{{nick.DATA}}今日蝉联“{{title.DATA}}”标识",
		"恭喜 {{user.DATA}} 升级至{{level.DATA}}",
	} {
		encoded, err := json.Marshal(template)
		require.NoError(t, err)
		for _, media := range []int{0, 1} {
			message, err := ParseMessage(fmt.Appendf(nil, `{"gid":1,"id":2,"type":321,"media_type":%d,"sub_type":101,"from_uid":5394206608,"template":%s}`, media, encoded))
			require.NoError(t, err)
			if media == 0 {
				require.Equal(t, EventGroupNotice, message.EventType)
				require.Empty(t, message.SenderID)
			} else {
				require.Equal(t, EventNone, message.EventType)
			}
			require.Nil(t, message.Content.EventActor)
		}
	}
	// Missing or different protocol discriminators must not classify templates.
	for _, fields := range []string{`"type":322,"media_type":0`, `"type":322,"media_type":0,"sub_type":102`, `"type":322,"media_type":1,"sub_type":101`} {
		message, err := ParseMessage(fmt.Appendf(nil, `{"gid":1,"id":2,"template":"{{name.DATA}} 加入了群",%s}`, fields))
		require.NoError(t, err)
		require.Equal(t, EventNone, message.EventType)
	}
}

func TestGroupEventsRemainSearchableWithoutSenderPreferences(t *testing.T) {
	store, db := testStore(t)
	ctx := context.Background()
	now := time.Now().UTC()
	messages := []Message{
		{GroupID: "1", MessageID: "1", SenderID: "42", Text: "match"},
		// Keep a stale sender to prove event semantics do not depend on the backfill.
		{GroupID: "1", MessageID: "2", SenderID: "42", EventType: EventMemberJoined, Text: "match", Content: MessageContent{EventActor: &EventActor{UID: "23", ScreenName: "新人"}}},
		{GroupID: "1", MessageID: "3", SenderID: "99", EventType: EventGroupNotice, Text: "match"},
	}
	for i := range messages {
		messages[i].Raw = []byte(`{}`)
		messages[i].FetchedAt = now
	}
	_, err := store.SaveMessages(ctx, messages)
	require.NoError(t, err)
	require.NoError(t, db.Create(&UserPreference{OwnerUsername: "reader", UID: "42", IsStarred: true, IsBlocked: true}).Error)
	page, err := store.ListMessages(ctx, "1", MessageFilter{OwnerUsername: "reader", Search: SearchQuery{Terms: []string{"match"}}, Limit: 50})
	require.NoError(t, err)
	require.Equal(t, []MessageID{"2", "3"}, messageIDs(page.Messages))
	page, err = store.ListMessages(ctx, "1", MessageFilter{SenderIDs: []string{"42", "99"}, Limit: 50})
	require.NoError(t, err)
	require.Equal(t, []MessageID{"1"}, messageIDs(page.Messages))
	starred := true
	blocked := false
	_, err = store.UpdateUserPreference(ctx, "reader", "42", UserPreferencePatch{IsStarred: &starred, IsBlocked: &blocked})
	require.NoError(t, err)
	page, err = store.ListMessages(ctx, "1", MessageFilter{OwnerUsername: "reader", StarredOnly: true, Limit: 50})
	require.NoError(t, err)
	require.Equal(t, []MessageID{"1"}, messageIDs(page.Messages))
	senders, err := store.ListSenders(ctx, "1")
	require.NoError(t, err)
	require.Len(t, senders, 1)
	require.Equal(t, "42", senders[0].UID)
	context, err := store.GetMessageContext(ctx, "1", "2", 1, 1)
	require.NoError(t, err)
	require.Equal(t, EventMemberJoined, context.Messages[1].EventType)
	require.Equal(t, messages[1].Content.EventActor, context.Messages[1].Content.EventActor)
	block, err := store.CreateBookmarkBlock(ctx, "reader", "events", nil, []MessageRef{{GroupID: "1", MessageID: "2"}})
	require.NoError(t, err)
	bookmarks, err := store.ListBookmarkMessages(ctx, "reader", block.ID, "", 50)
	require.NoError(t, err)
	require.Len(t, bookmarks.Messages, 1)
	require.Equal(t, EventMemberJoined, bookmarks.Messages[0].EventType)
}
