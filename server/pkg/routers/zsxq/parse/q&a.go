package parse

import (
	"context"
	"fmt"

	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
	"go.uber.org/zap"
)

func (s *ParseService) parseQA(logger *zap.Logger, topic *models.Topic) (authorID int, authorName string, err error) {
	question := topic.Question
	answer := topic.Answer
	if question == nil || answer == nil {
		return 0, "", fmt.Errorf("failed to parse q&a: no question or answer")
	}

	authorID, authorName, err = s.parseAuthor(&answer.Answerer)
	if err != nil {
		return 0, "", fmt.Errorf("failed to parse author: %w", err)
	}

	if err = s.saveImages(question.Images, topic.TopicID, topic.CreateTime, logger); err != nil {
		return 0, "", fmt.Errorf("failed to save question images: %w", err)
	}

	if err = s.saveImages(answer.Images, topic.TopicID, topic.CreateTime, logger); err != nil {
		return 0, "", fmt.Errorf("failed to save answer images: %w", err)
	}

	if err = s.saveVoice(logger, answer.Voice, topic.TopicID, topic.CreateTime); err != nil {
		return 0, "", fmt.Errorf("failed to save voice: %w", err)
	}

	return authorID, authorName, nil
}

func (s *ParseService) saveVoice(logger *zap.Logger, voice *models.Voice, topicID int, createTimeStr string) (err error) {
	if voice == nil {
		return nil
	}

	objectKey := fmt.Sprintf("zsxq/%d.%s", voice.VoiceID, "wav")
	resp, err := s.request.LimitStream(context.Background(), voice.URL, logger)
	if err != nil {
		return fmt.Errorf("failed to download voice %d: %w", voice.VoiceID, err)
	}
	if err = s.file.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
		return fmt.Errorf("failed to save voice %d: %w", voice.VoiceID, err)
	}

	// Get voice stream from file service,
	// then send it to ai service to get transcript
	voiceStream, err := s.file.GetStream(objectKey)
	if err != nil {
		return fmt.Errorf("failed to get voice stream: %w", err)
	}
	defer func() {
		_ = voiceStream.Close()
	}()

	transcript, err := s.ai.Text(voiceStream)
	if err != nil {
		return fmt.Errorf("failed to get transcript for voice %s: %w", objectKey, err)
	}

	polishedTranscript, err := s.ai.Polish(transcript)
	if err != nil {
		return fmt.Errorf("failed to polish transcript for voice %s: %w", objectKey, err)
	}

	createTime, err := zsxqTime.DecodeZsxqAPITime(createTimeStr)
	if err != nil {
		return fmt.Errorf("failed to decode create time: %w", err)
	}

	if err = s.db.SaveObjectInfo(&db.Object{
		ID:              voice.VoiceID,
		TopicID:         topicID,
		Time:            createTime,
		Type:            "voice",
		ObjectKey:       objectKey,
		StorageProvider: []string{s.file.AssetsDomain()},
		Transcript:      polishedTranscript,
		Url:             voice.URL,
	}); err != nil {
		return fmt.Errorf("failed to save voice info to database: %w", err)
	}

	return nil
}
