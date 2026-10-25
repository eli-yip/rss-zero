package weibochat

import (
	"context"
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
	highlight, err := parseHighlightQuery(c.QueryParams(), filter.Search)
	if err != nil {
		return httputil.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	owner, err := ownerUsername(c)
	if err != nil {
		return err
	}
	filter.OwnerUsername = owner
	ctx := c.Request().Context()
	page, err := h.store.ListMessages(ctx, groupID, filter)
	if err != nil {
		return err
	}
	items, err := h.messageResponses(ctx, owner, groupID, page.Messages, highlight)
	if err != nil {
		return err
	}
	result := MessageListResponse{Items: items, HasMore: page.HasMore}
	return c.JSON(http.StatusOK, httputil.NewResp("weibo messages", result))
}

func (h *Controller) messageResponses(ctx context.Context, owner, groupID string, messages []weibochat.Message, highlight weibochat.SearchQuery) ([]MessageResponse, error) {
	userIDs, assetKeys := make([]string, 0), make([]string, 0)
	for _, message := range messages {
		if message.SenderID != "" {
			userIDs = append(userIDs, message.SenderID)
		}
		for _, ref := range message.Content.Assets {
			assetKeys = append(assetKeys, ref.AssetKey)
		}
	}
	messageIDs := make([]string, 0, len(messages))
	for _, message := range messages {
		messageIDs = append(messageIDs, string(message.MessageID))
	}
	counts, err := h.store.GetBookmarkCounts(ctx, owner, groupID, messageIDs)
	if err != nil {
		return nil, err
	}
	preferences, err := h.store.GetUserPreferences(ctx, owner, userIDs)
	if err != nil {
		return nil, err
	}
	users, err := h.store.GetUsers(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	assets, err := h.store.GetAssets(ctx, groupID, assetKeys)
	if err != nil {
		return nil, err
	}
	byUser := make(map[string]SenderResponse, len(users))
	for _, user := range users {
		sender := senderResponse(user)
		sender.IsStarred = preferences[user.UID].IsStarred
		sender.IsBlocked = preferences[user.UID].IsBlocked
		byUser[user.UID] = sender
	}
	byAsset := make(map[string]weibochat.Asset, len(assets))
	for _, asset := range assets {
		byAsset[asset.AssetKey] = asset
	}
	items := make([]MessageResponse, 0, len(messages))
	for _, message := range messages {
		item := MessageResponse{
			BookmarkCount:   counts[string(message.MessageID)],
			IsSenderStarred: preferences[message.SenderID].IsStarred,
			IsSenderBlocked: preferences[message.SenderID].IsBlocked,
			TextMatches:     highlight.FindMatches(message.Text),
			GroupID:         message.GroupID, MessageID: string(message.MessageID), SenderID: message.SenderID,
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
				asset.Width, asset.Height = stored.Width, stored.Height
				if stored.Status == weibochat.AssetStored {
					asset.URL = storedURL(stored.StorageProvider, stored.ObjectKey)
				}
			}
			item.Assets = append(item.Assets, asset)
		}
		items = append(items, item)
	}
	return items, nil
}

func parseMessageFilter(query url.Values) (weibochat.MessageFilter, error) {
	filter := weibochat.MessageFilter{Limit: 50, SenderIDs: query["sender_id"], AssetKinds: query["asset_kind"]}
	starred, err := optionalBool(query, "starred_only")
	if err != nil {
		return filter, err
	}
	if starred != nil {
		filter.StarredOnly = *starred
	}
	search, err := parseSearchParameter(query, "q")
	if err != nil {
		return filter, err
	}
	filter.Search = search
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
	for _, name := range []string{"sent_from", "sent_before", "before_message_id", "after_message_id", "limit"} {
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
		case "before_message_id", "after_message_id":
			id := weibochat.MessageID(value)
			if !id.Valid() {
				return filter, fmt.Errorf("invalid %s", name)
			}
			if name == "before_message_id" {
				filter.BeforeMessageID = id
			} else {
				filter.AfterMessageID = id
			}
		case "limit":
			limit, err := strconv.Atoi(value)
			if err != nil || limit < 1 || limit > 100 {
				return filter, fmt.Errorf("limit must be between 1 and 100")
			}
			filter.Limit = limit
		}
	}
	if filter.BeforeMessageID != "" && filter.AfterMessageID != "" {
		return filter, fmt.Errorf("before_message_id and after_message_id are mutually exclusive")
	}
	if filter.SentFrom != nil && filter.SentBefore != nil && !filter.SentFrom.Before(*filter.SentBefore) {
		return filter, fmt.Errorf("sent_from must be before sent_before")
	}
	return filter, nil
}

func parseSearchParameter(query url.Values, name string) (weibochat.SearchQuery, error) {
	values, present := query[name]
	if !present {
		return weibochat.SearchQuery{}, nil
	}
	if len(values) != 1 {
		return weibochat.SearchQuery{}, fmt.Errorf("%s requires one value", name)
	}
	parsed, err := weibochat.ParseSearchQuery(values[0])
	if err != nil {
		return parsed, fmt.Errorf("%s: %w", name, err)
	}
	return parsed, nil
}

func parseHighlightQuery(query url.Values, fallback weibochat.SearchQuery) (weibochat.SearchQuery, error) {
	if _, present := query["highlight_q"]; !present {
		return fallback, nil
	}
	return parseSearchParameter(query, "highlight_q")
}
