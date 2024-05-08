//lint:file-ignore U1000 Ignore unused function temporarily for development
package parse

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"

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

func (ps *ParseService) downloadPic(picURL, objectKey string) (err error) {
	resp, err := ps.requestService.GetPicStream(picURL)
	if err != nil {
		return fmt.Errorf("failed to get pic stream: %w", err)
	}

	if err = ps.fileService.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
		return fmt.Errorf("failed to save image stream to file service: %w", err)
	}

	return nil
}

func (ps *ParseService) generateObjectKey(picURL string) (key string, err error) {
	u, err := url.Parse(picURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse pic url: %w", err)
	}
	return fmt.Sprintf("weibo/%s", path.Base(u.Path)), nil
}
