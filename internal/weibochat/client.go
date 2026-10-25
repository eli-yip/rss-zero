package weibochat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const (
	messagesURL  = "https://api.weibo.com/webim/groupchat/query_messages.json"
	chatReferer  = "https://api.weibo.com/chat"
	weiboSource  = "209678993"
	userAgent    = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/139.0.0.0 Safari/537.36"
	maxPageBytes = 16 << 20
)

type FetchMessagesRequest struct {
	GroupID string
	Before  MessageID // Empty means the latest page; otherwise an exclusive upper bound.
}

type MessagePage struct {
	Messages  []json.RawMessage
	FetchedAt time.Time
}

// Client reads credentials for each request so callers can replace an expired SUB.
// It never follows redirects for credential-bearing message requests.
type Client struct {
	http          *http.Client
	sub           func(context.Context) (string, error)
	invalidateSUB func(string)
}

func NewClient(client *http.Client, sub func(context.Context) (string, error)) *Client {
	if client == nil {
		client = &http.Client{}
	}
	copyClient := *client
	copyClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{http: &copyClient, sub: sub}
}

// APIError deliberately excludes upstream error text, which can contain private data.
type APIError struct{ Code string }

func (e *APIError) Error() string { return "weibo API rejected request (code " + e.Code + ")" }

func (c *Client) FetchMessages(ctx context.Context, input FetchMessagesRequest) (MessagePage, error) {
	if !MessageID(input.GroupID).Valid() || (input.Before != "" && !input.Before.Valid()) {
		return MessagePage{}, fmt.Errorf("invalid group ID or message cursor")
	}
	before := string(input.Before)
	if before == "" {
		before = "0"
	}
	query := url.Values{"id": {input.GroupID}, "count": {"20"}, "max_mid": {before}, "query_sender": {"1"}, "convert_emoji": {"1"}, "source": {weiboSource}, "t": {strconv.FormatInt(time.Now().UnixMilli(), 10)}}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, messagesURL+"?"+query.Encode(), nil)
	if err != nil {
		return MessagePage{}, err
	}
	if err = c.authorize(req); err != nil {
		return MessagePage{}, err
	}
	response, err := c.http.Do(req)
	if err != nil {
		return MessagePage{}, fmt.Errorf("fetch messages: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return MessagePage{}, fmt.Errorf("fetch messages: HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxPageBytes+1))
	if err != nil {
		return MessagePage{}, fmt.Errorf("read messages: %w", err)
	}
	if len(data) > maxPageBytes {
		return MessagePage{}, fmt.Errorf("message page exceeds size limit")
	}
	var envelope struct {
		Result    bool            `json:"result"`
		Messages  json.RawMessage `json:"messages"`
		ErrorCode json.RawMessage `json:"error_code"`
	}
	if err = json.Unmarshal(data, &envelope); err != nil {
		return MessagePage{}, fmt.Errorf("invalid message response JSON")
	}
	if !envelope.Result {
		code := "unknown"
		var id MessageID
		if json.Unmarshal(envelope.ErrorCode, &id) == nil {
			code = string(id)
		}
		if code == "21301" && c.invalidateSUB != nil {
			// Use the credential sent by this request, never reload a newer value.
			sent, _ := req.Cookie("SUB")
			c.invalidateSUB(sent.Value)
		}
		return MessagePage{}, &APIError{Code: code}
	}
	raw := bytes.TrimSpace(envelope.Messages)
	if len(raw) == 0 || raw[0] != '[' {
		return MessagePage{}, fmt.Errorf("message response lacks an array")
	}
	var messages []json.RawMessage
	if err = json.Unmarshal(raw, &messages); err != nil {
		return MessagePage{}, fmt.Errorf("invalid message array")
	}
	return MessagePage{Messages: messages, FetchedAt: time.Now().UTC()}, nil
}

func (c *Client) authorize(req *http.Request) error {
	if c.sub == nil {
		return fmt.Errorf("SUB provider is required")
	}
	sub, err := c.sub(req.Context())
	if err != nil {
		return fmt.Errorf("load SUB: %w", err)
	}
	if sub == "" {
		return fmt.Errorf("SUB is empty")
	}
	req.AddCookie(&http.Cookie{Name: "SUB", Value: sub})
	req.Header.Set("Referer", chatReferer)
	req.Header.Set("User-Agent", userAgent)
	return nil
}
