package weibochat

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/eli-yip/rss-zero/internal/weibochat"
	"github.com/eli-yip/rss-zero/pkg/httputil"
	"github.com/labstack/echo/v5"
)

func (h *Controller) ListMessages(c *echo.Context) error {
	groupID := c.Param("group_id")
	if !weibochat.MessageID(groupID).Valid() {
		return httputil.NewHTTPError(http.StatusBadRequest, "invalid group_id")
	}
	filter, err := parseMessageFilter(c.QueryParams())
	if err != nil {
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	ctx := c.Request().Context()
	page, err := h.store.ListMessages(ctx, groupID, filter)
	if err != nil {
		return err
	}
	userIDs, assetKeys := make([]string, 0), make([]string, 0)
	for _, message := range page.Messages {
		if message.SenderID != "" {
			userIDs = append(userIDs, message.SenderID)
		}
		for _, ref := range message.Content.Assets {
			assetKeys = append(assetKeys, ref.AssetKey)
		}
	}
	users, err := h.store.GetUsers(ctx, userIDs)
	if err != nil {
		return err
	}
	assets, err := h.store.GetAssets(ctx, groupID, assetKeys)
	if err != nil {
		return err
	}
	byUser := make(map[string]SenderResponse, len(users))
	for _, user := range users {
		byUser[user.UID] = senderResponse(user)
	}
	byAsset := make(map[string]weibochat.Asset, len(assets))
	for _, asset := range assets {
		byAsset[asset.AssetKey] = asset
	}
	result := MessageListResponse{Items: make([]MessageResponse, 0, len(page.Messages))}
	for _, message := range page.Messages {
		item := MessageResponse{
			GroupID: message.GroupID, MessageID: string(message.MessageID), SenderID: message.SenderID,
			SentAt: utc(message.SentAt), Text: message.Text, Links: append([]weibochat.Link{}, message.Content.Links...),
			Assets:      make([]AssetResponse, 0, len(message.Content.Assets)),
			MessageType: message.MessageType, MediaType: message.MediaType, SubType: message.SubType, ParseStatus: "parsed",
		}
		if user, ok := byUser[message.SenderID]; ok {
			item.Sender = &user
		}
		if message.ParseError != "" {
			item.ParseStatus = "partial"
		}
		for _, ref := range message.Content.Assets {
			asset := AssetResponse{AssetKey: ref.AssetKey, Kind: ref.Kind, Status: weibochat.AssetPending}
			if stored, ok := byAsset[ref.AssetKey]; ok {
				asset.Status, asset.ContentType, asset.ByteSize = stored.Status, stored.ContentType, stored.ByteSize
				if stored.Status == weibochat.AssetStored {
					asset.URL = storedURL(stored.StorageProvider, stored.ObjectKey)
				}
			}
			item.Assets = append(item.Assets, asset)
		}
		result.Items = append(result.Items, item)
	}
	if page.NextBeforeMessageID != "" {
		value := string(page.NextBeforeMessageID)
		result.NextBeforeMessageID = &value
	}
	return c.JSON(http.StatusOK, httputil.NewResp("weibo messages", result))
}

func parseMessageFilter(query url.Values) (weibochat.MessageFilter, error) {
	filter := weibochat.MessageFilter{Limit: 50, SenderIDs: query["sender_id"], AssetKinds: query["asset_kind"]}
	for _, id := range filter.SenderIDs {
		if !weibochat.MessageID(id).Valid() {
			return filter, fmt.Errorf("invalid sender_id")
		}
	}
	for _, kind := range filter.AssetKinds {
		if kind != "image" && kind != "video" && kind != "sticker" {
			return filter, fmt.Errorf("asset_kind must be image, video or sticker")
		}
	}
	for _, name := range []string{"sent_from", "sent_before", "before_message_id", "limit"} {
		values, present := query[name]
		if !present {
			continue
		}
		if len(values) != 1 || values[0] == "" {
			return filter, fmt.Errorf("%s requires one non-empty value", name)
		}
		value := values[0]
		switch name {
		case "sent_from", "sent_before":
			parsed, err := time.Parse(time.RFC3339, value)
			if err != nil {
				return filter, fmt.Errorf("%s must be RFC3339", name)
			}
			if name == "sent_from" {
				filter.SentFrom = &parsed
			} else {
				filter.SentBefore = &parsed
			}
		case "before_message_id":
			filter.BeforeMessageID = weibochat.MessageID(value)
			if !filter.BeforeMessageID.Valid() {
				return filter, fmt.Errorf("invalid before_message_id")
			}
		case "limit":
			limit, err := strconv.Atoi(value)
			if err != nil || limit < 1 || limit > 100 {
				return filter, fmt.Errorf("limit must be between 1 and 100")
			}
			filter.Limit = limit
		}
	}
	if filter.SentFrom != nil && filter.SentBefore != nil && !filter.SentFrom.Before(*filter.SentBefore) {
		return filter, fmt.Errorf("sent_from must be before sent_before")
	}
	return filter, nil
}
