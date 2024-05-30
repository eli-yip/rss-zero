package parse

import (
	"encoding/json"
	"fmt"

	apiModels "github.com/eli-yip/rss-zero/pkg/routers/weibo/parse/api_models"
	"go.uber.org/zap"
)

func (ps *ParseService) ParseTweetList(body []byte, logger *zap.Logger) (tweets []json.RawMessage, err error) {
	logger.Info("Start to parse tweet list")
	var apiResp apiModels.ApiResp
	if err = json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal body: %w", err)
	}

	if apiResp.OK != apiModels.OK {
		return nil, fmt.Errorf("response is not OK: %d", apiResp.OK)
	}

	logger.Info("Parse tweet list successfully")
	return apiResp.Data.List, nil
}
