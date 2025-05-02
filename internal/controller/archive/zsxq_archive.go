package archive

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/render"
	zsxqRender "github.com/eli-yip/rss-zero/pkg/routers/zsxq/render"
)

func (h *Controller) HandleZsxqWebTopic(link string) (result *archiveResult, err error) {
	// Supported link format:
	// https://wx.zsxq.com/dweb2/index/topic_detail/2855145852245441
	// https://wx.zsxq.com/group/28855218411241/topic/2855488118555511
	topicID, found := extractTopicIDFromLink(link)
	if !found {
		return nil, fmt.Errorf("unsupported link format: %s", link)
	}

	idInt, err := strconv.Atoi(topicID)
	if err != nil {
		return nil, fmt.Errorf("failed to convert topic id to int: %w", err)
	}

	topic, err := h.zsxqDBService.GetTopicByID(idInt)
	if err != nil {
		return nil, fmt.Errorf("failed to get topic by id: %w", err)
	}

	if isOldStyleZsxqLink(link) {
		return &archiveResult{redirectTo: render.BuildArchiveLink(config.C.Settings.ServerURL, zsxqRender.BuildLink(topic.GroupID, topic.ID))}, nil
	}

	topicToRender := &zsxqRender.Topic{
		ID:       idInt,
		GroupID:  topic.GroupID,
		Title:    topic.Title,
		Type:     topic.Type,
		Digested: topic.Digested,
		Time:     topic.Time,
		Text:     topic.Text,
	}

	fullTextMd, err := h.zsxqFullTextRenderService.FullText(topicToRender)
	if err != nil {
		return nil, fmt.Errorf("failed to render full text: %w", err)
	}

	html, err := h.htmlRender.Render(zsxqRender.BuildTitle(topicToRender), fullTextMd)
	if err != nil {
		return nil, fmt.Errorf("failed to render html: %w", err)
	}
	return &archiveResult{html: html}, nil
}

func isOldStyleZsxqLink(link string) bool {
	return strings.HasPrefix(link, "https://wx.zsxq.com/dweb2/index/topic_detail/")
}

func extractTopicIDFromLink(link string) (topicID string, found bool) {
	if strings.HasPrefix(link, "https://wx.zsxq.com/dweb2/index/topic_detail/") {
		return strings.TrimPrefix(link, "https://wx.zsxq.com/dweb2/index/topic_detail/"), true
	}

	re := regexp.MustCompile(`https://wx.zsxq.com/group/\d+/topic/(\d+)`)
	matches := re.FindStringSubmatch(link)
	if len(matches) == 2 {
		return matches[1], true
	}
	return "", false
}

func (h *Controller) HandleZsxqShareLink(link string) (result *archiveResult, err error) {
	link, err = getZsxqRealLink(link)
	if err != nil {
		return nil, fmt.Errorf("failed to get web link: %w", err)
	}

	return h.HandleZsxqWebTopic(link)
}

// getZsxqRealLink get the real link of zsxq share link
func getZsxqRealLink(link string) (string, error) {
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

	// Though zsxq has changed its web link, but share link 302 location is still like below
	// location https://wx.zsxq.com/mweb/views/topicdetail/topicdetail.html?topic_id=2855145852245441&inviter_id=815528414188822&inviter_sid=b48m2w8mk1&keyword=6WBoJ
	params := location.Query()
	topicID := params.Get("topic_id")
	return fmt.Sprintf("https://wx.zsxq.com/dweb2/index/topic_detail/%s", topicID), nil
}
