package parse

import (
	"fmt"
	"strconv"

	dbModels "github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/db/models"
	"github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/parse/models"
	zsxqTime "github.com/eli-yip/zsxq-parser/pkg/routers/zsxq/time"
)

func (s *ParseService) parseQA(topic *models.Topic) (authorName string, err error) {
	question := topic.Question
	answer := topic.Answer
	if question == nil || answer == nil {
		return "", nil
	}

	authorName, err = s.parseAuthor(&answer.Answerer)
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

	return authorName, nil
}

func (s *ParseService) parseVoice(voice *models.Voice, topicID int, createTimeStr string) (err error) {
	if voice == nil {
		return nil
	}

	objectKey := fmt.Sprintf("zsxq/%d.%s", voice.VoiceID, "wav")
	resp, err := s.Request.WithLimiterStream(voice.URL)
	if err != nil {
		return err
	}
	if err = s.File.SaveStream(objectKey, resp.Body); err != nil {
		return err
	}

	// Get voice stream from file service,
	// then send it to ai service to get transcript
	voiceStream, err := s.File.Get(strconv.Itoa(voice.VoiceID))
	if err != nil {
		return err
	}
	defer voiceStream.Close()

	transcript, err := s.AI.Text(voiceStream)
	if err != nil {
		return err
	}
	polishedTranscript, err := s.AI.Polish(transcript)
	if err != nil {
		return err
	}

	createTime, err := zsxqTime.DecodeStringToTime(createTimeStr)
	if err != nil {
		return err
	}

	if err = s.DB.SaveObjectInfo(&dbModels.Object{
		ID:              voice.VoiceID,
		TopicID:         topicID,
		Time:            createTime,
		Type:            "voice",
		ObjectKey:       objectKey,
		StorageProvider: []string{s.File.GetAssetsDomain()},
		Transcript:      polishedTranscript,
	}); err != nil {
		return err
	}

	return nil
}
