package archive

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	zsxqRender "github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
	"gorm.io/gorm"
)

func (h *Controller) HandleZsxqWebTopic(link string) (html string, err error) {
	// https://wx.zsxq.com/dweb2/index/topic_detail/2855145852245441
	topicID := strings.TrimPrefix(link, "https://wx.zsxq.com/dweb2/index/topic_detail/")
	idInt, err := strconv.Atoi(topicID)
	if err != nil {
		return "", fmt.Errorf("failed to convert topic id to int: %w", err)
	}

	topic, err := h.zsxqDBService.GetTopicByID(idInt)
	if err != nil {
		return "", fmt.Errorf("failed to get topic by id: %w", err)
	}

	topicToRender := &zsxqRender.Topic{
		ID:        idInt,
		Title:     topic.Title,
		Type:      topic.Type,
		Digested:  topic.Digested,
		Time:      topic.Time,
		ShareLink: topic.ShareLink,
		Text:      topic.Text,
	}
	fullTextMd, err := h.zsxqFullTextRenderService.FullText(topicToRender)
	if err != nil {
		return "", fmt.Errorf("failed to render full text: %w", err)
	}

	html, err = h.htmlRender.Render(zsxqRender.BuildTitle(topicToRender), fullTextMd)
	if err != nil {
		return "", fmt.Errorf("failed to render html: %w", err)
	}
	return html, nil
}

func (h *Controller) HandleZsxqShareLink(link string) (html string, err error) {
	topic, err := h.zsxqDBService.GetTopicIDByShareLink(link)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", fmt.Errorf("failed to get topic id by share link: %w", err)
	} else if errors.Is(err, gorm.ErrRecordNotFound) {
		link, err = getWebLink(link)
		if err != nil {
			return "", fmt.Errorf("failed to get web link: %w", err)
		}
	} else {
		link = fmt.Sprintf("https://wx.zsxq.com/mweb/views/topicdetail/topicdetail.html?topic_id=%d", topic)
	}

	return h.HandleZsxqWebTopic(link)
}

func getWebLink(link string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, link, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/127.0.0.0 Safari/537.36")
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get link: %w", err)
	}
	if resp.StatusCode != http.StatusFound {
		return "", fmt.Errorf("failed to get link, status code: %d", resp.StatusCode)
	}
	location, err := resp.Location()
	if err != nil {
		return "", fmt.Errorf("failed to get location: %w", err)
	}
	// location https://wx.zsxq.com/mweb/views/topicdetail/topicdetail.html?topic_id=2855145852245441&inviter_id=815528414188822&inviter_sid=b48m2w8mk1&keyword=6WBoJ
	params := location.Query()
	topicID := params.Get("topic_id")
	return fmt.Sprintf("https://wx.zsxq.com/dweb2/index/topic_detail/%s", topicID), nil
}
