//lint:file-ignore U1000 Ignore all unused code for developing
package parse

import (
	"encoding/json"
	"fmt"

	"github.com/eli-yip/rss-zero/internal/md"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/weibo/parse/api_models"
)

func (ps *ParseService) parseTweet(tweet apiModels.Tweet) (text string, err error) {
	text = tweet.TextRaw

	if len(tweet.PicIDs) == 0 {
		text, err = ps.getLongText(tweet.MBlogID)
		if err != nil {
			return "", fmt.Errorf("failed to get long text: %w", err)
		}
	}

	text += "\n\n"

	for _, picID := range tweet.PicIDs {
		picInfo := tweet.PicInfos[picID]
		objectKey, err := ps.generateObjectKey(picID)
		if err != nil {
			return "", fmt.Errorf("failed to generate object key for %s: %w", picID, err)
		}

		if err = ps.savePic(picInfo.Original.URL, objectKey); err != nil {
			return "", fmt.Errorf("failed to save pic: %w", err)
		}

		if err = ps.savePicInfo(tweet.ID, picID, picInfo.Original.URL, objectKey); err != nil {
			return "", fmt.Errorf("failed to save pic info: %w", err)
		}

		text += md.Image(objectKey, ps.fileService.AssetsDomain()+objectKey) + "\n\n"
		// text += md.Image(objectKey, `https://image.com/`+objectKey) + "\n\n"
	}

	return trimRightNewLine(text), nil
}

func (ps *ParseService) getLongText(mBlogID string) (text string, err error) {
	u := fmt.Sprintf("https://weibo.com/ajax/statuses/longtext?id=%s", mBlogID)
	data, err := ps.requestService.LimitRaw(u)
	if err != nil {
		return "", fmt.Errorf("failed to get long text: %w", err)
	}

	var longTextResp apiModels.LongTextApiResp
	if err = json.Unmarshal(data, &longTextResp); err != nil {
		return "", fmt.Errorf("failed to unmarshal long text response: %w", err)
	}

	return longTextResp.Data.LongTextContent, nil
}
