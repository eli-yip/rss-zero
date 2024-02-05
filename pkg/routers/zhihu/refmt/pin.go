package refmt

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/eli-yip/rss-zero/internal/md"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
	"go.uber.org/zap"
)

// refmtPin formats all pins in db for the authorID
func (s *RefmtService) refmtPin(authorID string) (err error) {
	s.logger.Info("start to format pins", zap.String("author_id", authorID))

	var latestTime time.Time
	latestTime, err = s.db.GetLatestPinTime(authorID)
	if err != nil {
		s.logger.Error("fail to get latest pin time in db", zap.Error(err))
		return err
	}
	if latestTime.IsZero() {
		s.logger.Info("no pin in db, finish formatting")
		return nil
	}
	latestTime = latestTime.Add(time.Second) // add 1 second to lastTime to avoid result missing

	var wg sync.WaitGroup
	var count int64
	idSet := mapset.NewSet[int]()
	for {
		if latestTime.Before(longLongAgo) {
			s.logger.Info("latest time long long ago, break")
			break
		}

		var pins []db.Pin
		if pins, err = s.db.FetchNPin(defaultFetchLimit, db.FetchPinOption{
			FetchOptionBase: db.FetchOptionBase{
				UserID:    &authorID,
				StartTime: time.Time{},
				EndTime:   latestTime,
			},
		}); err != nil {
			s.logger.Info("fail to fetch pin from db",
				zap.Error(err), zap.String("author_id", authorID),
				zap.Time("end_time", latestTime), zap.Int("limit", defaultFetchLimit))
		}
		if len(pins) == 0 {
			s.logger.Info("there no more pins, break")
			break
		}
		s.logger.Info("fetch pins from db successfully",
			zap.Int("count", len(pins)),
			zap.Time("end_time", latestTime), zap.Int("limit", defaultFetchLimit))

		for i, p := range pins {
			p := p
			idSet.Add(p.ID)
			wg.Add(1)
			latestTime = p.CreateAt

			go func(i int, p *db.Pin) {
				defer wg.Done()

				atomic.AddInt64(&count, 1)
				logger := s.logger.With(zap.Int("pin_id", p.ID))
				logger.Info("start to format pin")

				var pin apiModels.Pin
				if err := json.Unmarshal(p.Raw, &pin); err != nil {
					logger.Error("fail to unmarshal pin", zap.Error(err))
					return
				}

				text, err := s.parsePinContent(pin.Content, logger)
				if err != nil {
					logger.Error("fail to parse pin content", zap.Error(err))
					return
				}

				formattedText, err := s.mdfmt.FormatStr(text)
				if err != nil {
					logger.Error("fail to format markdown text", zap.Error(err))
					return
				}

				if err = s.db.SavePin(&db.Pin{
					ID:       p.ID,
					AuthorID: p.AuthorID,
					CreateAt: p.CreateAt,
					Text:     formattedText,
					Raw:      p.Raw,
				}); err != nil {
					logger.Error("fail to save pin to db", zap.Error(err))
					return
				}

				logger.Info("save pin to db successfully")
				logger.Info("format pin successfully")
			}(i, &p)
		}
	}

	wg.Wait()

	return nil
}

func (s *RefmtService) parsePinContent(content []json.RawMessage, logger *zap.Logger) (output string, err error) {
	textPart := make([]string, 0)

	for _, c := range content {
		var contentType apiModels.PinContentType
		if err := json.Unmarshal(c, &contentType); err != nil {
			return "", err
		}

		switch contentType.Type {
		case "text":
			logger.Info("find text content")
			text := ""

			var textContent apiModels.PinContentText
			if err := json.Unmarshal(c, &textContent); err != nil {
				return "", err
			}
			textBytes, err := s.htmlConvert.Convert([]byte(textContent.Content))
			if err != nil {
				return "", err
			}
			text = strings.ReplaceAll(string(textBytes), `\|`, "\n\n")
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

			picID := parse.URLToID(imageContent.OriginalURL)
			object, err := s.db.GetObjectInfo(picID)
			if err != nil {
				logger.Error("fail to get object info from db", zap.Error(err))
				return "", err
			}

			objectURL := fmt.Sprintf("%s/%s", object.StorageProvider[0], object.ObjectKey)
			text = fmt.Sprintf("![%s](%s)", object.ObjectKey, objectURL)

			textPart = append(textPart, text)
		case "link":
			logger.Info("find link content")

			var linkContent apiModels.PinLink
			if err := json.Unmarshal(c, &linkContent); err != nil {
				return "", err
			}
			text := fmt.Sprintf("[%s](%s)", linkContent.Title, linkContent.URL)

			textPart = append(textPart, text)
		case "video":
		default:
			logger.Info("find unknown content type", zap.String("type", contentType.Type))
		}
	}

	return md.Join(textPart...), nil
}
