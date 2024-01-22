package parse

import (
	"fmt"

	dbModels "github.com/eli-yip/rss-zero/pkg/routers/zsxq/db/models"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
	"go.uber.org/zap"
)

func (s *ParseService) parseQA(topic *models.Topic) (authorID int, authorName string, err error) {
	question := topic.Question
	answer := topic.Answer
	if question == nil || answer == nil {
		s.log.Info("no question or answer in this topic", zap.Int("topic_id", topic.TopicID))
		return 0, "", nil
	}

	authorID, authorName, err = s.parseAuthor(&answer.Answerer)
	if err != nil {
		return 0, "", err
	}

	if err = s.parseImages(question.Images, topic.TopicID, topic.CreateTime); err != nil {
		s.log.Error("failed to parse images", zap.Error(err))
		return 0, "", err
	}

	if err = s.parseImages(answer.Images, topic.TopicID, topic.CreateTime); err != nil {
		s.log.Error("failed to parse images", zap.Error(err))
		return 0, "", err
	}

	if err = s.parseVoice(answer.Voice, topic.TopicID, topic.CreateTime); err != nil {
		s.log.Error("failed to parse voice", zap.Error(err))
		return 0, "", err
	}

	return authorID, authorName, nil
}

func (s *ParseService) parseVoice(voice *models.Voice, topicID int, createTimeStr string) (err error) {
	if voice == nil {
		return nil
	}

	objectKey := fmt.Sprintf("zsxq/%d.%s", voice.VoiceID, "wav")
	resp, err := s.Request.LimitStream(voice.URL)
	if err != nil {
		return err
	}
	if err = s.File.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
		return err
	}
	s.log.Info("voice saved", zap.String("object_key", objectKey))

	// Get voice stream from file service,
	// then send it to ai service to get transcript
	s.log.Info("get voice stream", zap.String("object_key", objectKey))
	voiceStream, err := s.File.Get(objectKey)
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

	createTime, err := zsxqTime.DecodeZsxqAPITime(createTimeStr)
	if err != nil {
		return err
	}

	if err = s.DB.SaveObjectInfo(&dbModels.Object{
		ID:              voice.VoiceID,
		TopicID:         topicID,
		Time:            createTime,
		Type:            "voice",
		ObjectKey:       objectKey,
		StorageProvider: []string{s.File.AssetsDomain()},
		Transcript:      polishedTranscript,
	}); err != nil {
		return err
	}

	return nil
}
