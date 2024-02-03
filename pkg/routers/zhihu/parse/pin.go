package parse

import (
	"encoding/json"
	"strconv"
	"time"

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

	text, err = p.parseHTML(pin.HTML, pinID, logger)
	if err != nil {
		return "", err
	}
	logger.Info("parse html successfully")

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

	if err = p.db.SavePin(&db.Pin{
		ID:       pinID,
		AuthorID: pin.Author.ID,
		CreateAt: time.Unix(pin.CreateAt, 0),
		Text:     formattedText,
		Raw:      content,
	}); err != nil {
		return "", err
	}

	return formattedText, nil
}
