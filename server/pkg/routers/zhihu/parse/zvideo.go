package parse

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/eli-yip/rss-zero/config"
	"github.com/eli-yip/rss-zero/pkg/routers/zhihu/db"
	apiModels "github.com/eli-yip/rss-zero/pkg/routers/zhihu/parse/api_models"
)

type Zvideo struct {
	// Filename is the name of the file that the video will be saved to,
	// it's built from the title and the time of the video.
	// This filename has no extension name.
	Filename string
	// Url is the url of the video, it will be used to download the video.
	Url string
}

type ZvideoParser interface {
	// ParseZvideoList parse the zvideo list and return the list of zvideos that need to be downloaded.
	ParseZvideoList(content []byte, logger *zap.Logger) ([]Zvideo, error)
}

type ZvideoParseService struct{ db db.DB }

func NewZvideoParseService(db db.DB) ZvideoParser { return &ZvideoParseService{db: db} }

func (z *ZvideoParseService) ParseZvideoList(content []byte, logger *zap.Logger) (list []Zvideo, err error) {
	list = make([]Zvideo, 0)

	var latestZvideoPublishedAtinDB time.Time
	latestZvideoInDB, err := z.db.GetLatestZvideo()
	if err != nil {
		logger.Error("Failed to get the latest zvideo in db", zap.Error(err))
		return nil, err
	}
	if latestZvideoInDB == nil {
		logger.Info("No zvideo in db, will parse all zvideos")
		latestZvideoPublishedAtinDB = time.Date(2000, 9, 30, 0, 0, 0, 0, config.C.BJT) // Set to long time ago
	} else {
		latestZvideoPublishedAtinDB = latestZvideoInDB.PublishedAt
	}

	zvideos := &apiModels.ZvideoList{}
	if err = json.Unmarshal(content, zvideos); err != nil {
		logger.Error("Failed to decode zvideo list", zap.Error(err))
		return nil, err
	}
	logger.Info("Unmarshal zvideo list successfully", zap.Int("count", len(zvideos.Data)))

	for _, zr := range zvideos.Data {
		var zvideo apiModels.Zvideo
		if err = json.Unmarshal(zr, &zvideo); err != nil {
			logger.Error("Failed to decode zvideo", zap.Error(err))
			return nil, err
		}

		zvideoPublishedAt := parseUnixTime(zvideo.PublishedAt)
		if !zvideoPublishedAt.After(latestZvideoPublishedAtinDB) ||
			zvideoPublishedAt.Before(time.Date(2024, 9, 30, 0, 0, 0, 0, config.C.BJT)) {
			// Used for testing
			// zvideoPublishedAt.Before(time.Date(2022, 8, 29, 0, 0, 0, 0, config.C.BJT)) {
			continue
		}

		var (
			bestBitrate float64 = 0
			bestStreem          = ""
		)

		if zvideo.Video.PlayList != nil {
			for _, stream := range *zvideo.Video.PlayList {
				if stream.Bitrate > bestBitrate {
					bestBitrate = stream.Bitrate
					bestStreem = stream.PlayUrl
				}
			}
		}

		if zvideo.Video.PlayListV2 != nil {
			for _, stream := range *zvideo.Video.PlayListV2 {
				if stream.Bitrate > bestBitrate {
					bestBitrate = stream.Bitrate
					bestStreem = stream.PlayUrl
				}
			}
		}

		filename := timeToFilenameTime(zvideoPublishedAt) + cleanTitle(zvideo.Title)

		list = append(list, Zvideo{
			Filename: filename,
			Url:      bestStreem,
		})

		if err = z.db.SaveZvideoInfo(&db.Zvideo{
			ID:          zvideo.ID,
			PublishedAt: zvideoPublishedAt,
			Filename:    filename,
			Raw:         zr,
		}); err != nil {
			logger.Error("Failed to save zvideo to db", zap.Error(err))
			return nil, fmt.Errorf("failed to save zvideo info to db: %w", err)
		}
	}

	return list, nil
}

func parseUnixTime(unixTime int) time.Time { return time.Unix(int64(unixTime), 0) }

// timeToFilenameTime converts a time.Time to a string that can be used in filename.
//
// The format is "YYMMDD".
func timeToFilenameTime(t time.Time) string { return t.Format("060102") }

func cleanTitle(t string) string {
	replaceMap := map[string]string{
		" - 知乎":  "",
		"「直播回放」": "",
		"---":    "",
		"？":      "",
		"。":      "",
		"、":      ",",
		"：":      ":",
		"，":      ",",
		":":      "：",
	}

	for k, v := range replaceMap {
		t = strings.ReplaceAll(t, k, v)
	}

	return t
}
