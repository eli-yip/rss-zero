package parse

import (
	"errors"
	"fmt"

	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
	"go.uber.org/zap"
)

func (s *ParseService) saveImages(images []models.Image, topicID int, createTimeStr string, logger *zap.Logger) (err error) {
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
					return errors.New("found no valid image url")
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
		resp, err := s.request.LimitStream(url, logger)
		if err != nil {
			return fmt.Errorf("failed to download image %d: %w", image.ImageID, err)
		}
		if err = s.file.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
			return fmt.Errorf("failed to save image %d: %w", image.ImageID, err)
		}

		createTime, err := zsxqTime.DecodeZsxqAPITime(createTimeStr)
		if err != nil {
			return fmt.Errorf("failed to decode create time: %w", err)
		}

		if err = s.db.SaveObjectInfo(&db.Object{
			ID:              image.ImageID,
			TopicID:         topicID,
			Time:            createTime,
			Type:            "image",
			ObjectKey:       objectKey,
			StorageProvider: []string{s.file.AssetsDomain()},
		}); err != nil {
			return fmt.Errorf("failed to save image info to database: %w", err)
		}
	}

	return nil
}
