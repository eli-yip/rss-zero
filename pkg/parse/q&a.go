package parse

import (
	"strconv"

	"github.com/eli-yip/zsxq-parser/pkg/parse/models"
)

func (s *ParseService) parseQA(question *models.Question, answer *models.Answer) (author string, err error) {
	if question == nil || answer == nil {
		return "", nil
	}

	author, err = s.parseAuthor(&answer.Answerer)
	if err != nil {
		return "", err
	}

	if err = s.parseImages(question.Images); err != nil {
		return "", err
	}
	if err = s.parseImages(answer.Images); err != nil {
		return "", err
	}

	if err = s.parseVoice(answer.Voice); err != nil {
		return "", err
	}

	return author, nil
}

func (s *ParseService) parseVoice(voice *models.Voice) (err error) {
	if err = s.FileService.Save(strconv.Itoa(voice.VoiceID), voice.URL); err != nil {
		return err
	}

	return nil
}
