package parse

import (
	"context"
	"fmt"

	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/db"
	"github.com/eli-yip/rss-zero/pkg/routers/zsxq/parse/models"
	zsxqTime "github.com/eli-yip/rss-zero/pkg/routers/zsxq/time"
	"go.uber.org/zap"
)

// parseQA 抽取一条 q&a 的全部待提交事实（作者 / 提问 + 回答图片 + 语音对象），不落库。
// 沿用旧行为：缺 question 或 answer 报错（ParseTopic 据此 error，不存 root）。
func (s *ParseService) parseQA(logger *zap.Logger, topic *models.Topic) (author *db.Author, objects []db.Object, err error) {
	question := topic.Question
	answer := topic.Answer
	if question == nil || answer == nil {
		return nil, nil, fmt.Errorf("failed to parse q&a: no question or answer")
	}

	author = buildAuthor(&answer.Answerer)

	questionImages, err := s.collectImages(question.Images, topic.TopicID, topic.CreateTime, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to save question images: %w", err)
	}
	objects = append(objects, questionImages...)

	answerImages, err := s.collectImages(answer.Images, topic.TopicID, topic.CreateTime, logger)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to save answer images: %w", err)
	}
	objects = append(objects, answerImages...)

	voiceObject, err := s.collectVoice(logger, answer.Voice, topic.TopicID, topic.CreateTime)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to save voice: %w", err)
	}
	if voiceObject != nil {
		objects = append(objects, *voiceObject)
	}

	return author, objects, nil
}

// collectVoice 下载语音转存 OSS 并转写（事务外网络 + AI 副作用），返回待提交的对象事实行；
// 不落库。voice 为 nil 时返回 nil。
func (s *ParseService) collectVoice(logger *zap.Logger, voice *models.Voice, topicID int, createTimeStr string) (*db.Object, error) {
	if voice == nil {
		return nil, nil
	}

	objectKey := fmt.Sprintf("zsxq/%d.%s", voice.VoiceID, "wav")
	resp, err := s.request.LimitStream(context.Background(), voice.URL, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to download voice %d: %w", voice.VoiceID, err)
	}
	if err = s.file.SaveStream(objectKey, resp.Body, resp.ContentLength); err != nil {
		return nil, fmt.Errorf("failed to save voice %d: %w", voice.VoiceID, err)
	}

	// Get voice stream from file service,
	// then send it to ai service to get transcript
	voiceStream, err := s.file.GetStream(objectKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get voice stream: %w", err)
	}
	defer voiceStream.Close()

	transcript, err := s.ai.Text(voiceStream)
	if err != nil {
		return nil, fmt.Errorf("failed to get transcript for voice %s: %w", objectKey, err)
	}

	polishedTranscript, err := s.ai.Polish(transcript)
	if err != nil {
		return nil, fmt.Errorf("failed to polish transcript for voice %s: %w", objectKey, err)
	}

	createTime, err := zsxqTime.DecodeZsxqAPITime(createTimeStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode create time: %w", err)
	}

	return &db.Object{
		ID:              voice.VoiceID,
		TopicID:         topicID,
		Time:            createTime,
		Type:            "voice",
		ObjectKey:       objectKey,
		StorageProvider: []string{s.file.AssetsDomain()},
		Transcript:      polishedTranscript,
		Url:             voice.URL,
	}, nil
}
