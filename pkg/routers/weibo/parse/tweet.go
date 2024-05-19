//lint:file-ignore U1000 Ignore unused function temporarily for development
package parse

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/routers/weibo/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/weibo/parse/api_models"
)

func (ps *ParseService) ParseTweet(content []byte) (text string, err error) {
	tweet := apiModels.Tweet{}
	if err = json.Unmarshal(content, &tweet); err != nil {
		return "", fmt.Errorf("failed to unmarshal content to tweet: %w", err)
	}
	logger := ps.logger.With(zap.Int("tweet_id", tweet.ID))
	logger.Info("start to parse tweet")

	text, err = ps.buildText(tweet)
	if err != nil {
		return "", fmt.Errorf("failed to parse text: %w", err)
	}
	logger.Info("parse text successfully")

	formattedText, err := ps.mdfmt.FormatStr(text)
	if err != nil {
		return "", fmt.Errorf("failed to format text: %w", err)
	}
	logger.Info("format text successfully")

	tweetTime, err := parseTime(tweet.CreatedAt)
	if err != nil {
		return "", fmt.Errorf("failed to parse time: %w", err)
	}

	if err = ps.dbService.SaveTweet(&db.Tweet{
		ID:        tweet.ID,
		MBlogID:   tweet.MBlogID,
		CreatedAt: tweetTime,
		AuthorID:  tweet.User.ID,
		Text:      formattedText,
		Raw:       content,
	}); err != nil {
		return "", fmt.Errorf("failed to save tweet: %w", err)
	}

	return formattedText, nil
}

func parseTime(timeStr string) (time.Time, error) {
	const layout = "Mon Jan 02 15:04:05 -0700 2006"
	t, err := time.Parse(layout, timeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse time: %w", err)
	}
	return t.In(config.C.BJT), nil
}

func trimRightNewLine(text string) string { return strings.TrimRight(text, "\n") }
