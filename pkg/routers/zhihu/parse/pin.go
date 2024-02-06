package parse

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
	"go.uber.org/zap"
)

func (p *Parser) ParsePinList(content []byte, index int) (paging apiModels.Paging, pins []apiModels.Pin, err error) {
	logger := p.logger.With(zap.Int("pin list page", index))

	pinList := apiModels.PinList{}
	if err = json.Unmarshal(content, &pinList); err != nil {
		return apiModels.Paging{}, nil, err
	}
	logger.Info("unmarshal pin list successfully")

	return pinList.Paging, pinList.Data, nil
}

// ParsePin parses the zhihu.com/api/v4 resp
func (p *Parser) ParsePin(content []byte) (text string, err error) {
	pin := apiModels.Pin{}
	if err = json.Unmarshal(content, &pin); err != nil {
		return "", err
	}
	pinID, err := strconv.Atoi(pin.ID)
	if err != nil {
		return "", err
	}
	logger := p.logger.With(zap.Int("pin_id", pinID))
	logger.Info("unmarshal pin successfully")

	text, err = p.parsePinContent(pin.Content, pinID, logger)
	if err != nil {
		return "", err
	}
	logger.Info("parse html successfully")

	if text == "" {
		logger.Info("no text content found")
		return "", nil
	}

	formattedText, err := p.mdfmt.FormatStr(text)
	if err != nil {
		return "", err
	}
	logger.Info("format markdown text successfully")

	if err = p.db.SaveAuthor(&db.Author{
		ID:   pin.Author.ID,
		Name: pin.Author.Name,
	}); err != nil {
		return "", err
	}
	logger.Info("save author to db successfully")

	title, err := p.ai.Conclude(formattedText)
	if err != nil {
		logger.Error("failed to conclude", zap.Error(err))
		err = fmt.Errorf("failed to conclude: %w", err)
		return "", err
	}

	if err = p.db.SavePin(&db.Pin{
		ID:       pinID,
		AuthorID: pin.Author.ID,
		CreateAt: time.Unix(pin.CreateAt, 0),
		Title:    title,
		Text:     formattedText,
		Raw:      content,
	}); err != nil {
		return "", err
	}

	return formattedText, nil
}

func (p *Parser) parsePinContent(content []json.RawMessage, id int, logger *zap.Logger) (text string, err error) {
	textPart := make([]string, 0)

	for _, c := range content {
		var contentType apiModels.PinContentType
		if err := json.Unmarshal(c, &contentType); err != nil {
			return "", err
		}

		switch contentType.Type {
		case "text":
			text := ""
			logger.Info("find text content")

			var textContent apiModels.PinContentText
			if err := json.Unmarshal(c, &textContent); err != nil {
				return "", err
			}
			textBytes, err := p.htmlToMarkdown.Convert([]byte(textContent.Content))
			if err != nil {
				return "", err
			}
			text += string(textBytes)
			text = strings.ReplaceAll(text, `\|`, "\n\n")

			textPart = append(textPart, text)

			logger.Info("convert html to markdown successfully")
		case "image":
			logger.Info("find image content")
			text := ""
			var imageContent apiModels.PinImage
			if err := json.Unmarshal(c, &imageContent); err != nil {
				return "", err
			}
			logger = logger.With(zap.String("url", imageContent.OriginalURL))

			picID := URLToID(imageContent.OriginalURL)

			resp, err := p.request.NoLimitStream(imageContent.OriginalURL)
			if err != nil {
				return "", err
			}
			logger.Info("get image stream succussfully")

			const zhihuImageObjectKeyLayout = "zhihu/%d.jpg"
			objectKey := fmt.Sprintf(zhihuImageObjectKeyLayout, picID)
			if err = p.file.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
				return "", err
			}
			logger.Info("save image stream to file service successfully", zap.String("object_key", objectKey))

			if err = p.db.SaveObjectInfo(&db.Object{
				ID:              picID,
				Type:            db.ObjectImageType,
				ContentType:     db.ContentTypeAnswer,
				ContentID:       id,
				ObjectKey:       objectKey,
				URL:             imageContent.OriginalURL,
				StorageProvider: []string{p.file.AssetsDomain()},
			}); err != nil {
				return "", err
			}
			logger.Info("save object info to db successfully")

			objectURL := fmt.Sprintf("%s/%s", p.file.AssetsDomain(), objectKey)
			text = fmt.Sprintf("![%s](%s)", objectKey, objectURL)

			textPart = append(textPart, text)

			logger.Info("convert image to markdown successfully")
		case "link":
			logger.Info("find link content")

			var linkContent apiModels.PinLink
			if err := json.Unmarshal(c, &linkContent); err != nil {
				return "", err
			}
			text = fmt.Sprintf("[%s](%s)", linkContent.Title, linkContent.URL)

			textPart = append(textPart, text)
		case "video":
		default:
			fmt.Println(string(c))
			return "", fmt.Errorf("unknown content type: %s", contentType.Type)
		}
	}

	text = md.Join(textPart...)
	return text, nil
}
