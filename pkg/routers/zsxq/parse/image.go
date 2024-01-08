package parse

import (
	"errors"
	"fmt"

	zsxqTime "github.com/eli-yip/zsxq-parser/internal/time"
	dbModels "github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db/models"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/parse/models"
)

func (s *ParseService) parseImages(images []models.Image, topicID int, createTimeStr string) (err error) {
	if images == nil {
		return nil
	}

	for _, image := range images {
		var url string
		switch image.Original {
		case nil:
			switch image.Large {
			case nil:
				switch image.Thumbnail {
				case nil:
					return errors.New("no image")
				default:
					url = image.Thumbnail.URL
				}
			default:
				url = image.Large.URL
			}
		default:
			url = image.Original.URL
		}
		objectKey := fmt.Sprintf("%d.%s", image.ImageID, image.Type)
		resp, err := s.RequestService.WithLimiterStream(url)
		if err != nil {
			return err
		}
		if err = s.FileService.SaveHTTPStream(objectKey, resp); err != nil {
			return err
		}

		createTime, err := zsxqTime.DecodeStringToTime(createTimeStr)
		if err != nil {
			return err
		}

		if err = s.DBService.SaveObject(&dbModels.Object{
			ID:              image.ImageID,
			TopicID:         topicID,
			Time:            createTime,
			ObjectKey:       objectKey,
			StorageProvider: []string{s.FileService.GetAssetsDomain()},
			Type:            "image",
		}); err != nil {
			return err
		}
	}

	return nil
}
