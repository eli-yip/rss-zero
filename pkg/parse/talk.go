package parse

import (
	"errors"
	"strconv"

	"github.com/eli-yip/zsxq-parser/pkg/parse/models"
)

var ErrNoText = errors.New("no text in this topic")

func (s *ParseService) parseTalk(talk *models.Talk) (author string, err error) {
	if talk == nil || talk.Text == nil {
		return "", ErrNoText
	}

	author, err = s.parseAuthor(&talk.Owner)
	if err != nil {
		return "", err
	}

	if err = s.parseFiles(talk.Files); err != nil {
		return "", err
	}

	if err = s.parseImage(talk.Images); err != nil {
		return "", err
	}

	return author, nil
}

func (s *ParseService) parseFiles(files []models.File) (err error) {
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
	}

	return nil
}
