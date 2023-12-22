package parse

import (
	"errors"
	"strconv"

	zsxqTime "github.com/eli-yip/zsxq-parser/internal/time"
	dbModels "github.com/eli-yip/zsxq-parser/pkg/db/models"
	"github.com/eli-yip/zsxq-parser/pkg/parse/models"
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
		if err = s.FileService.Save(strconv.Itoa(image.ImageID), url); err != nil {
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
			StorageProvider: []string{s.FileService.GetAssetsDomain()},
			Type:            "image",
		}); err != nil {
			return err
		}
	}

	return nil
}
