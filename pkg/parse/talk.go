package parse

import (
	"errors"
	"strconv"

	zsxqTime "github.com/eli-yip/zsxq-parser/internal/time"
	dbModels "github.com/eli-yip/zsxq-parser/pkg/db/models"
	"github.com/eli-yip/zsxq-parser/pkg/parse/models"
)

var ErrNoText = errors.New("no text in this topic")

func (s *ParseService) parseTalk(topic *models.Topic) (author string, err error) {
	talk := topic.Talk
	if talk == nil || talk.Text == nil {
		return "", ErrNoText
	}

	author, err = s.parseAuthor(&talk.Owner)
	if err != nil {
		return "", err
	}

	if err = s.parseFiles(talk.Files, topic.TopicID, topic.CreateTime); err != nil {
		return "", err
	}

	if err = s.parseImages(talk.Images, topic.TopicID, topic.CreateTime); err != nil {
		return "", err
	}

	return author, nil
}

func (s *ParseService) parseFiles(files []models.File, topicID int, createTimeStr string) (err error) {
	if files == nil {
		return nil
	}

	for _, file := range files {
		downloadLink, err := s.FileService.DownloadLink(file.FileID)
		if err != nil {
			return err
		}

		if err = s.FileService.Save(strconv.Itoa(file.FileID), downloadLink); err != nil {
			return err
		}

		createTime, err := zsxqTime.DecodeStringToTime(createTimeStr)
		if err != nil {
			return err
		}

		if err = s.DBService.SaveObject(&dbModels.Object{
			ID:              file.FileID,
			TopicID:         topicID,
			Time:            createTime,
			Type:            "file",
			StorageProvider: []string{s.FileService.GetAssetsDomain()},
		}); err != nil {
			return err
		}
	}

	return nil
}
