package parse

import (
	"errors"
	"fmt"

	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
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
		objectKey := fmt.Sprintf("zsxq/%d.%s", image.ImageID, image.Type)
		resp, err := s.request.LimitStream(url)
		if err != nil {
			return err
		}
		if err = s.file.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
			return err
		}

		createTime, err := zsxqTime.DecodeZsxqAPITime(createTimeStr)
		if err != nil {
			return err
		}

		if err = s.db.SaveObjectInfo(&db.Object{
			ID:              image.ImageID,
			TopicID:         topicID,
			Time:            createTime,
			ObjectKey:       objectKey,
			StorageProvider: []string{s.file.AssetsDomain()},
			Type:            "image",
		}); err != nil {
			return err
		}
	}

	return nil
}
