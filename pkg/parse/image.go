package parse

import (
	"errors"
	"strconv"

	"github.com/eli-yip/zsxq-parser/pkg/parse/models"
)

func (s *ParseService) parseImage(images []models.Image) (err error) {
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
	}

	return nil
}
