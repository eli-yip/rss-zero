package weibochat

import (
	"encoding/json"
	"net/url"
)

// EventType distinguishes confirmed group events from ordinary messages.
// Unknown upstream types and templates remain ordinary archived messages.
type EventType string

const (
	EventNone            EventType = ""
	EventMemberJoined    EventType = "member_joined"
	EventMessageRecalled EventType = "message_recalled"
	EventGroupNotice     EventType = "group_notice"
	EventWeiboPublished  EventType = "weibo_published"
)

// EventActor is the person associated with an event, not its chat sender.
// Names retain the original event observation and do not use user preferences.
type EventActor struct {
	UID        string `json:"uid"`
	ScreenName string `json:"screen_name"`
}

func classifyEvent(wire wireMessage) (EventType, *EventActor) {
	if wire.Type == nil || wire.MediaType == nil || *wire.MediaType != 0 || wire.SubType == nil || *wire.SubType != 101 {
		return EventNone, nil
	}
	var template string
	if len(wire.Template) > 0 && json.Unmarshal(wire.Template, &template) != nil {
		return EventNone, nil
	}
	switch *wire.Type {
	case 322:
		if template == "{{name.DATA}} 加入了群" {
			var data struct {
				Name struct {
					Value  string `json:"value"`
					Scheme string `json:"scheme"`
				} `json:"name"`
			}
			_ = json.Unmarshal(wire.Data, &data)
			name := data.Name
			var actor *EventActor
			if parsed, err := url.Parse(name.Scheme); err == nil && parsed.Scheme == "sinaweibo" && parsed.Host == "userinfo" {
				uid := parsed.Query().Get("uid")
				if MessageID(uid).Valid() {
					actor = &EventActor{UID: uid, ScreenName: name.Value}
				}
			}
			return EventMemberJoined, actor
		}
	case 321:
		if wire.SenderID == "5394206608" {
			switch template {
			case "涉及资金问题请务必提高警惕，谨防诈骗。{{cases.DATA}}",
				"恭喜{{nick.DATA}}今日获得“{{title.DATA}}”标识",
				"恭喜{{nick.DATA}}今日蝉联“{{title.DATA}}”标识",
				"恭喜 {{user.DATA}} 升级至{{level.DATA}}":
				return EventGroupNotice, nil
			}
		}
	case 344:
		if template == "" && wire.User != nil && wire.User.Name != "" && wire.Text == wire.User.Name+" 撤回了一条消息" {
			return EventMessageRecalled, &EventActor{UID: string(wire.SenderID), ScreenName: wire.User.Name}
		}
	case 100:
		if wire.Text == "我发布了新微博，快来围观吧！" {
			var actor *EventActor
			if wire.User != nil && wire.SenderID != "" {
				actor = &EventActor{UID: string(wire.SenderID), ScreenName: wire.User.Name}
			}
			return EventWeiboPublished, actor
		}
	}
	return EventNone, nil
}
