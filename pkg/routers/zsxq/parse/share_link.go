package parse

import (
	"encoding/json"
	"fmt"
)

type ShareAPIResponse struct {
	RespData struct {
		ShareURL string `json:"share_url"`
	} `json:"resp_data"`
}

const ZsxqShareLinkAPIBaseURL = "https://api.zsxq.com/v2/topics/%d/share_url"

func (p *ParseService) shareLink(topicID int) (link string, err error) {
	url := fmt.Sprintf(ZsxqShareLinkAPIBaseURL, topicID)
	respByte, err := p.RequestService.WithLimiter(url)
	if err != nil {
		return "", err
	}
	var resp ShareAPIResponse
	if err = json.Unmarshal(respByte, &resp); err != nil {
		return "", err
	}
	return resp.RespData.ShareURL, err
}
