package weibochat

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

type wireMessage struct {
	SenderID  MessageID `json:"from_uid"`
	Time      *int64    `json:"time"`
	Type      *int      `json:"type"`
	MediaType *int      `json:"media_type"`
	SubType   *int      `json:"sub_type"`
	Text      string    `json:"content"`
	User      struct {
		Name        string `json:"screen_name"`
		Avatar      string `json:"profile_image_url"`
		LargeAvatar string `json:"avatar_large"`
	} `json:"from_user"`
	Files    []MessageID     `json:"fids"`
	Pictures []string        `json:"pids"`
	PicInfos json.RawMessage `json:"pic_infos"`
	URLs     []struct {
		Original string `json:"url_ori"`
		Info     struct {
			Short string `json:"url_short"`
			Long  string `json:"url_long"`
		} `json:"info"`
		Status json.RawMessage `json:"status"`
	} `json:"url_objects"`
}

// ParseMessage rejects invalid identities but retains messages whose content cannot
// be parsed, with ParseError populated. Unknown types remain available in Raw.
func ParseMessage(raw json.RawMessage) (Message, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return Message{}, fmt.Errorf("message must be a JSON object")
	}
	var group, id MessageID
	if err := json.Unmarshal(fields["gid"], &group); err != nil {
		return Message{}, fmt.Errorf("message group ID: %w", err)
	}
	if err := json.Unmarshal(fields["id"], &id); err != nil {
		return Message{}, fmt.Errorf("message ID: %w", err)
	}
	delete(fields, "attitude_info")
	cleaned, err := json.Marshal(fields)
	if err != nil {
		return Message{}, err
	}
	message := Message{GroupID: string(group), MessageID: id, Raw: cleaned}
	var wire wireMessage
	if err := json.Unmarshal(cleaned, &wire); err != nil {
		message.ParseError = "decode message content: " + err.Error()
		return message, nil
	}
	message.SenderID = string(wire.SenderID)
	if wire.Time != nil {
		sent := time.Unix(*wire.Time, 0).UTC()
		message.SentAt = &sent
	}
	message.MessageType, message.MediaType, message.SubType = wire.Type, wire.MediaType, wire.SubType
	message.Text = wire.Text
	message.Content.SenderName = wire.User.Name
	message.Content.AvatarURL = wire.User.LargeAvatar
	if message.Content.AvatarURL == "" {
		message.Content.AvatarURL = wire.User.Avatar
	}
	for _, item := range wire.URLs {
		message.Content.Links = append(message.Content.Links, Link{item.Original, item.Info.Short, item.Info.Long})
	}
	if wire.MediaType == nil {
		return message, nil
	}
	switch *wire.MediaType {
	case 1, 10:
		kind := "image"
		if *wire.MediaType == 10 {
			kind = "video"
		}
		for _, fid := range wire.Files {
			message.Content.Assets = append(message.Content.Assets, AssetRef{Kind: kind, AssetKey: "file/" + string(fid)})
		}
	case 15:
		message.Content.Assets, err = pictureRefs(wire.PicInfos, wire.Pictures, "sticker")
	case 14:
		for _, item := range wire.URLs {
			if len(item.Status) == 0 || string(item.Status) == "null" {
				continue
			}
			var status struct {
				IDs  []string        `json:"pic_ids"`
				Pics json.RawMessage `json:"pic_infos"`
			}
			if err = json.Unmarshal(item.Status, &status); err != nil {
				break
			}
			var refs []AssetRef
			refs, err = pictureRefs(status.Pics, status.IDs, "image")
			if err != nil {
				break
			}
			message.Content.Assets = append(message.Content.Assets, refs...)
		}
	}
	if err != nil {
		message.ParseError = "decode message pictures: " + err.Error()
	}
	return message, nil
}

func pictureRefs(raw json.RawMessage, ids []string, kind string) ([]AssetRef, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var pictures []json.RawMessage
	var keys []string
	if strings.HasPrefix(strings.TrimSpace(string(raw)), "[") {
		if err := json.Unmarshal(raw, &pictures); err != nil {
			return nil, err
		}
		keys = ids
	} else {
		var entries map[string]json.RawMessage
		if err := json.Unmarshal(raw, &entries); err != nil {
			return nil, err
		}
		keys = append([]string(nil), ids...)
		seen := map[string]bool{}
		for _, id := range keys {
			seen[id] = true
		}
		var remaining []string
		for id := range entries {
			if !seen[id] {
				remaining = append(remaining, id)
			}
		}
		sort.Strings(remaining)
		keys = append(keys, remaining...)
		for _, id := range keys {
			pictures = append(pictures, entries[id])
		}
	}
	var refs []AssetRef
	for i, rawPic := range pictures {
		if len(rawPic) == 0 {
			continue
		}
		var pic struct {
			OriginalPic string `json:"original_pic"`
			Original    struct {
				URL string `json:"url"`
			} `json:"original"`
			Largest struct {
				URL string `json:"url"`
			} `json:"largest"`
		}
		if err := json.Unmarshal(rawPic, &pic); err != nil {
			return refs, err
		}
		source := pic.OriginalPic
		if source == "" {
			source = pic.Largest.URL
		}
		if source == "" {
			source = pic.Original.URL
		}
		if source == "" {
			continue
		} // Never substitute a thumbnail.
		parsed, err := url.Parse(source)
		if err != nil || parsed.Hostname() == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
			return refs, fmt.Errorf("invalid original picture URL")
		}
		key := fmt.Sprintf("url/%x", sha256.Sum256([]byte(source)))
		if i < len(keys) && safePictureID(keys[i]) {
			key = "pic/" + keys[i]
		}
		refs = append(refs, AssetRef{Kind: kind, AssetKey: key, SourceURL: source})
	}
	return refs, nil
}

func safePictureID(id string) bool {
	if id == "" {
		return false
	}
	for _, c := range id {
		if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' && c != '-' {
			return false
		}
	}
	return true
}
