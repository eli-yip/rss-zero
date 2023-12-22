package parse

import (
	"strconv"

	zsxqTime "github.com/eli-yip/zsxq-parser/internal/time"
	dbModels "github.com/eli-yip/zsxq-parser/pkg/db/models"
	"github.com/eli-yip/zsxq-parser/pkg/parse/models"
)

func (s *ParseService) parseQA(topic *models.Topic) (author string, err error) {
	question := topic.Question
	answer := topic.Answer
	if question == nil || answer == nil {
		return "", nil
	}

	author, err = s.parseAuthor(&answer.Answerer)
	if err != nil {
		return "", err
	}

	if err = s.parseImages(question.Images, topic.TopicID, topic.CreateTime); err != nil {
		return "", err
	}
	if err = s.parseImages(answer.Images, topic.TopicID, topic.CreateTime); err != nil {
		return "", err
	}

	if err = s.parseVoice(answer.Voice, topic.TopicID, topic.CreateTime); err != nil {
		return "", err
	}

	return author, nil
}

func (s *ParseService) parseVoice(voice *models.Voice, topicID int, createTimeStr string) (err error) {
	if err = s.FileService.Save(strconv.Itoa(voice.VoiceID), voice.URL); err != nil {
		return err
	}

	voiceStream, err := s.FileService.Get(strconv.Itoa(voice.VoiceID))
	if err != nil {
		return err
	}
	defer voiceStream.Close()

	transcript, err := s.AIService.Text(voiceStream)
	if err != nil {
		return err
	}
	polishedTranscript, err := s.AIService.Polish(transcript)
	if err != nil {
		return err
	}

	createTime, err := zsxqTime.DecodeStringToTime(createTimeStr)
	if err != nil {
		return err
	}

	if err = s.DBService.SaveObject(&dbModels.Object{
		ID:              voice.VoiceID,
		TopicID:         topicID,
		Time:            createTime,
		Type:            "voice",
		StorageProvider: []string{s.FileService.GetAssetsDomain()},
		Transcript:      polishedTranscript,
	}); err != nil {
		return err
	}

	return nil
}
