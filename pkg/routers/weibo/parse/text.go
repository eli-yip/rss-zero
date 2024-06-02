//lint:file-ignore U1000 Ignore all unused code for developing
package parse

import (
	"encoding/json"
	"fmt"

	"github.com/eli-yip/rss-zero/internal/md"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/weibo/parse/api_models"
)

// buildText builds the text part for database field `text`
func (ps *ParseService) buildText(tweet apiModels.Tweet) (text string, err error) {
	if text, err = ps.buildTextPart(tweet.TextRaw, tweet.MBlogID, tweet.IsLongText); err != nil {
		return "", fmt.Errorf("failed to build text part: %w", err)
	}
	text += "\n\n"

	if tweet.ReTweetedStatus != nil {
		if retweetedPart, err := ps.buildRetweetPart(*tweet.ReTweetedStatus); err != nil {
			return "", fmt.Errorf("failed to build retweet part: %w", err)
		} else {
			text += retweetedPart
		}
	}
	text += "\n\n"

	if picPart, err := ps.buildPicPart(tweet.ID, tweet.PicIDs, tweet.PicInfos); err != nil {
		return "", fmt.Errorf("failed to build pic part: %w", err)
	} else {
		text += picPart
	}

	return trimRightNewLine(text), nil
}

func (ps *ParseService) buildTextPart(textRaw, mBlogID string, isLongText bool) (text string, err error) {
	text = textRaw

	if !isLongText {
		return
	}

	if text, err = ps.getLongText(mBlogID); err != nil {
		return "", fmt.Errorf("failed to get long text: %w", err)
	}

	return text, nil
}

func (ps *ParseService) buildRetweetPart(retweet apiModels.Tweet) (text string, err error) {
	return ps.buildText(retweet)
}

func (ps *ParseService) buildPicPart(tweetID int, picIDs []string, picInfos map[string]apiModels.PicInfo) (picPart string, err error) {
	for _, picID := range picIDs {
		picInfo := picInfos[picID]
		objectKey, err := ps.buildObjectKey(picInfo.Original.URL)
		if err != nil {
			return "", fmt.Errorf("failed to generate object key for %s: %w", picID, err)
		}

		if err = ps.savePic(picInfo.Original.URL, objectKey); err != nil {
			return "", fmt.Errorf("failed to save pic: %w", err)
		}

		if err = ps.savePicInfo(tweetID, picID, picInfo.Original.URL, objectKey); err != nil {
			return "", fmt.Errorf("failed to save pic info: %w", err)
		}

		picPart += md.Image(objectKey, ps.fileService.AssetsDomain()+objectKey) + "\n\n"
		// picPart += md.Image(objectKey, `https://image.com/`+objectKey) + "\n\n"
	}

	return trimRightNewLine(picPart), nil
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
