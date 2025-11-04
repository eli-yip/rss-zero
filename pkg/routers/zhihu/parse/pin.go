package parse

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/common"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
	"github.com/samber/lo"
)

type PinParser interface {
	ParsePinList(content []byte, index int, logger *zap.Logger) (paging apiModels.Paging, pinsExcerpt []apiModels.Pin, pins []json.RawMessage, err error)
	ParsePin(content []byte, logger *zap.Logger) (text string, err error)
}

func (p *ParseService) ParsePinList(content []byte, index int, logger *zap.Logger) (paging apiModels.Paging, pinsExcerpt []apiModels.Pin, pins []json.RawMessage, err error) {
	logger.Info("Start to parse pin list", zap.Int("pin_list_page_index", index))

	pinList := apiModels.PinList{}
	if err = json.Unmarshal(content, &pinList); err != nil {
		return apiModels.Paging{}, nil, nil, fmt.Errorf("failed to unmarshal content data in to pin list: %w", err)
	}
	logger.Info("Unmarshal pin list successfully")

	for _, rawMessage := range pinList.Data {
		pin := apiModels.Pin{}
		if err = json.Unmarshal(rawMessage, &pin); err != nil {
			return apiModels.Paging{}, nil, nil, fmt.Errorf("failed to unmarshal data in to pin: %w", err)
		}
		pinsExcerpt = append(pinsExcerpt, pin)
	}

	return pinList.Paging, pinsExcerpt, pinList.Data, nil
}

// ParsePin parses the zhihu.com/api/v4 resp
func (p *ParseService) ParsePin(content []byte, logger *zap.Logger) (text string, err error) {
	pin := apiModels.Pin{}
	if err = json.Unmarshal(content, &pin); err != nil {
		return emptyString, fmt.Errorf("failed to unmarshal content data in to pin: %w", err)
	}
	pinID, err := strconv.Atoi(pin.ID)
	if err != nil {
		return emptyString, fmt.Errorf("failed to convert pin id to int: %w", err)
	}
	logger.Info("Unmarshal pin successfully")

	pinInDB, exist, err := checkPinExist(pinID, p.db)
	if err != nil {
		return emptyString, fmt.Errorf("failed to check pin exist: %w", err)
	}
	if exist {
		if pinInDB.UpdateAt.IsZero() {
			logger.Info("Pin already exist, updated_at is zero, skip this pin")
			return pinInDB.Text, nil
		}
		pinUpdateAt := time.Unix(pin.UpdateAt, 0)
		if pinUpdateAt.After(pinInDB.UpdateAt) {
			logger.Info("Pin already exist, but updated_at is newer, update this pin")
		} else {
			logger.Info("Pin already exist, but updated_at is older, skip this pin")
			return pinInDB.Text, nil
		}
	}

	text, err = p.parseAndSavePin(&pin, content, pinID, logger)
	if err != nil {
		return "", fmt.Errorf("failed to parse and save pin: %w", err)
	}
	logger.Info("Parse and save pin successfully")

	return text, nil
}

func (p *ParseService) parseAndSavePin(pin *apiModels.Pin, content []byte, pinID int, logger *zap.Logger) (text string, err error) {
	var title string
	title, text, err = p.parsePinContent(pin.Content, pinID, logger)
	if err != nil {
		return "", fmt.Errorf("failed to parse pin content: %w", err)
	}
	logger.Info("Parse pin content successfully")

	var oText string
	if pin.OriginPin != nil {
		contentBytes, err := json.Marshal(pin.OriginPin)
		if err != nil {
			return "", fmt.Errorf("failed to marshal origin pin: %w", err)
		}
		oPinID, err := strconv.Atoi(pin.OriginPin.ID)
		if err != nil {
			return "", fmt.Errorf("failed to convert origin pin id to int: %w", err)
		}
		oText, err = p.parseAndSavePin(pin.OriginPin, contentBytes, oPinID, logger)
		if err != nil {
			return "", fmt.Errorf("failed to parse origin pin content: %w", err)
		}
		const originPinLayout = `这篇想法引用了另一篇想法：`
		oLink := fmt.Sprintf("https://www.zhihu.com/pin/%d", oPinID)
		oPinArchiveLink := fmt.Sprintf("%s/api/v1/archive/%s", config.C.Settings.ServerURL, oLink)
		oPinArchiveLink = fmt.Sprintf("[存档](%s)", oPinArchiveLink)
		oPinLink := fmt.Sprintf("[原文](%s)", oLink)
		oText = md.Join(originPinLayout, oText, oPinArchiveLink, oPinLink)
		oText = md.Quote(oText)
	}
	text = md.Join(text, oText)

	if text == "" {
		logger.Info("Found text content, return")
		return "", nil
	}

	formattedText, err := p.mdfmt.FormatStr(text)
	if err != nil {
		return "", fmt.Errorf("failed to format markdown text: %w", err)
	}
	logger.Info("Format markdown text successfully")

	if err = p.db.SaveAuthor(&db.Author{
		ID:   pin.Author.ID,
		Name: pin.Author.Name,
	}); err != nil {
		return "", fmt.Errorf("failed to save author info to db: %w", err)
	}
	logger.Info("Save author info to db successfully")

	if title == "" {
		if title, err = p.ai.Conclude(formattedText); err != nil {
			return "", fmt.Errorf("failed to conclude pin content: %w", err)
		}
		logger.Info("Conclude pin content successfully", zap.String("title", title))
	}

	if err = p.db.SavePin(&db.Pin{
		ID:       pinID,
		AuthorID: pin.Author.ID,
		CreateAt: time.Unix(pin.CreateAt, 0),
		UpdateAt: time.Unix(pin.UpdateAt, 0),
		Title:    title,
		Text:     formattedText,
		Raw:      content,
	}); err != nil {
		return "", fmt.Errorf("failed to save pin info to db: %w", err)
	}
	logger.Info("Save pin to db successfully")

	return formattedText, nil
}

func (p *ParseService) parsePinContent(content []json.RawMessage, id int, logger *zap.Logger) (title, text string, err error) {
	textPart := make([]string, 0)

	for _, c := range content {
		var contentType apiModels.PinContentType
		if err = json.Unmarshal(c, &contentType); err != nil {
			return emptyString, emptyString, fmt.Errorf("failed to unmarshal content type: %w", err)
		}

		switch contentType.Type {
		case "text":
			logger.Info("Found text content")

			var textContent apiModels.PinContentText
			if err = json.Unmarshal(c, &textContent); err != nil {
				return emptyString, emptyString, fmt.Errorf("failed to unmarshal text content: %w", err)
			}
			textBytes, err := p.htmlToMarkdown.Convert([]byte(textContent.Content))
			if err != nil {
				return emptyString, emptyString, fmt.Errorf("failed to convert html to markdown: %w", err)
			}
			text += string(textBytes)
			title, text = tryToFindTitle(text)

			textPart = append(textPart, text)

			logger.Info("Convert text part to markdown successfully")
		case "image":
			logger.Info("Found image content")
			var imageContent apiModels.PinImage
			if err = json.Unmarshal(c, &imageContent); err != nil {
				return emptyString, emptyString, fmt.Errorf("failed to unmarshal image content: %w", err)
			}
			picID := URLToID(imageContent.OriginalURL)

			resp, err := p.GetImageStream(imageContent.OriginalURL, logger)
			if err != nil {
				return emptyString, emptyString, fmt.Errorf("failed to get image %s stream: %w", imageContent.OriginalURL, err)
			}
			logger.Info("Get image stream succussfully")

			const zhihuImageObjectKeyLayout = "zhihu/%d.jpg"
			objectKey := fmt.Sprintf(zhihuImageObjectKeyLayout, picID)
			if err = p.file.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
				return emptyString, emptyString, fmt.Errorf("failed to save image stream %s to file service: %w", imageContent.OriginalURL, err)
			}
			logger.Info("Save image stream to file service successfully", zap.String("object_key", objectKey))

			if err = p.db.SaveObjectInfo(&db.Object{
				ID:              picID,
				Type:            db.ObjectTypeImage,
				ContentType:     common.TypeZhihuPin,
				ContentID:       id,
				ObjectKey:       objectKey,
				URL:             imageContent.OriginalURL,
				StorageProvider: []string{p.file.AssetsDomain()},
			}); err != nil {
				return emptyString, emptyString, fmt.Errorf("failed to save object info to db: %w", err)
			}
			logger.Info("Save object info to db successfully")

			objectURL := fmt.Sprintf("%s/%s", p.file.AssetsDomain(), objectKey)
			text = fmt.Sprintf("![%s](%s)", objectKey, objectURL)

			textPart = append(textPart, text)

			logger.Info("Convert image to markdown successfully")
		case "link":
			logger.Info("Found link content")

			var linkContent apiModels.PinLink
			if err := json.Unmarshal(c, &linkContent); err != nil {
				return emptyString, emptyString, fmt.Errorf("failed to unmarshal link content: %w", err)
			}
			text = fmt.Sprintf("[%s](%s)", linkContent.Title, linkContent.URL)

			textPart = append(textPart, text)
		case "video":
			logger.Info("Found video content")

			var videoContent apiModels.PinVideo
			if err := json.Unmarshal(c, &videoContent); err != nil {
				return emptyString, emptyString, fmt.Errorf("failed to unmarshal video content: %w", err)
			}
			logger.Info("Unmarshal video content successfully")

			maxVideo := lo.MaxBy(videoContent.Playlist, func(a, b apiModels.PlaylistItem) bool { return a.Size > b.Size })
			logger.Info("Found max video", zap.Any("max_video", maxVideo))

			videoURL := maxVideo.Url
			videoID := videoContent.VideoID
			logger.Info("Found video", zap.String("video_url", videoURL), zap.String("video_id", videoID))

			text = fmt.Sprintf("![视频 %s](%s)", videoID, videoURL)
			textPart = append(textPart, text)
		case "link_card":
			logger.Info("Found linkercard content")
		case "poll":
			logger.Info("Found poll content")
		default:
			return "", "", fmt.Errorf("unknown content type: %s", contentType.Type)
		}
	}

	text = md.Join(textPart...)
	return title, text, nil
}

func tryToFindTitle(text string) (title, content string) {
	var found bool
	title, content, found = strings.Cut(text, `\|`)
	if found {
		return title, content
	}
	return "", text
}

func checkPinExist(articleID int, db db.DB) (pin *db.Pin, exist bool, err error) {
	pin, err = db.GetPin(articleID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("failed to get pin from db: %w", err)
	}
	return pin, true, nil
}
