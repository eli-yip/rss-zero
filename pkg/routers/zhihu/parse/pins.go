package parse

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
	"go.uber.org/zap"
)

func (p *Parser) ParsePins(content []byte) (err error) {
	initialData, err := p.getInitialData(content)
	if err != nil {
		return err
	}
	p.logger.Info("get initial data successfully")

	var htmlPin apiModels.HTMLPin
	if err = json.Unmarshal([]byte(initialData), &htmlPin); err != nil {
		return err
	}
	p.logger.Info("unmarshal initial data successfully")

	for _, pin := range htmlPin.InitialState.Entities.Pins {
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
