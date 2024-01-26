package parse

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
	"go.uber.org/zap"
)

func (p *Parser) SplitPins(content []byte) ([]apiModels.Pin, error) {
	initialData, err := p.getInitialData(content)
	if err != nil {
		return nil, err
	}
	p.logger.Info("get initial data successfully")

	var htmlPin apiModels.HTMLPin
	if err = json.Unmarshal([]byte(initialData), &htmlPin); err != nil {
		return nil, err
	}
	p.logger.Info("unmarshal initial data successfully")

	pins := make([]apiModels.Pin, len(htmlPin.InitialState.Entities.Pins))
	for _, pin := range htmlPin.InitialState.Entities.Pins {
		pins = append(pins, pin)
	}
	return pins, nil
}

func (p *Parser) ParsePins(pins []apiModels.Pin) (err error) {
	for _, pin := range pins {
		pinID, err := strconv.Atoi(pin.ID)
		if err != nil {
			return err
		}

		logger := p.logger.With(zap.Int("pin_id", pinID))

		text, err := p.parseContent([]byte(pin.Content), pinID, logger)
		if err != nil {
			return err
		}
		logger.Info("parse content successfully")

		content, err := p.mdfmt.Format([]byte(text))
		if err != nil {
			return err
		}
		logger.Info("format content successfully")

		if exist, err := p.db.CheckAuthorExist(pin.AuthorID); err != nil || !exist {
			logger.Error("author not exist", zap.Error(err))
			return err
		}

		if err = p.db.SavePin(&db.Pin{
			ID:          pinID,
			AuthorID:    pin.AuthorID,
			CreatedTime: time.Unix(int64(pin.CreatedTime), 0),
			Text:        string(content),
		}); err != nil {
			return err
		}
	}

	return nil
}
