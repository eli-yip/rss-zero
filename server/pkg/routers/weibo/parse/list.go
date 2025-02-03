package parse

import (
	"encoding/json"
	"fmt"

	apiModels "github.com/eli-yip/rss-zero/pkg/routers/weibo/parse/api_models"
)

func (ps *ParseService) ParseTweetList(body []byte) (tweets []json.RawMessage, err error) {
	var apiResp apiModels.ApiResp
	if err = json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal body: %w", err)
	}

	if apiResp.OK != apiModels.OK {
		return nil, fmt.Errorf("response is not OK: %d", apiResp.OK)
	}

	return apiResp.Data.List, nil
}
