package parse

import (
	"context"
	"errors"
	"fmt"

	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
	"go.uber.org/zap"
)

// collectImages 下载图片转存 OSS（事务外网络副作用），返回待提交的对象事实行；不落库。
// 保留旧 saveImages 的 url 优选与错误路径：无有效 url 报错、下载/转存失败即中止整条 topic。
func (s *ParseService) collectImages(images []models.Image, topicID int, createTimeStr string, logger *zap.Logger) (objects []db.Object, err error) {
	if images == nil {
		return nil, nil
	}

	for _, image := range images {
		var url string
		switch image.Original {
		case nil:
			switch image.Large {
			case nil:
				switch image.Thumbnail {
				case nil:
					return nil, errors.New("found no valid image url")
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
		resp, err := s.request.LimitStream(context.Background(), url, logger)
		if err != nil {
			return nil, fmt.Errorf("failed to download image %d: %w", image.ImageID, err)
		}
		if err = s.file.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
			return nil, fmt.Errorf("failed to save image %d: %w", image.ImageID, err)
		}

		createTime, err := zsxqTime.DecodeZsxqAPITime(createTimeStr)
		if err != nil {
			return nil, fmt.Errorf("failed to decode create time: %w", err)
		}

		objects = append(objects, db.Object{
			ID:              image.ImageID,
			TopicID:         topicID,
			Time:            createTime,
			Type:            "image",
			ObjectKey:       objectKey,
			StorageProvider: []string{s.file.AssetsDomain()},
			Url:             url,
		})
	}

	return objects, nil
}
