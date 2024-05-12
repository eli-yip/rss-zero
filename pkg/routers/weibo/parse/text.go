//lint:file-ignore U1000 Ignore all unused code for developing
package parse

import (
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/weibo/parse/api_models"
)

func parseText(tweet apiModels.Tweet) (text string, err error) {
	return tweet.TextRaw, nil
}
