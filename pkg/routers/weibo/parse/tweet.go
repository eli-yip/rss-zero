//lint:file-ignore U1000 Ignore unused function temporarily for development
package parse

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"

	apiModels "github.com/eli-yip/rss-zero/pkg/routers/weibo/parse/api_models"
)

func (ps *ParseService) ParseTweet(content []byte) (text string, err error) {
	tweet := apiModels.Tweet{}
	if err = json.Unmarshal(content, &tweet); err != nil {
		return "", fmt.Errorf("failed to unmarshal content to tweet: %w", err)
	}
	logger := ps.logger.With(zap.Int("tweet_id", tweet.ID))
	logger.Info("start to parse tweet")

	panic("not implemented")
}
