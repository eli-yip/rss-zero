package ai

import (
	"context"
	"io"

	"github.com/sashabaranov/go-openai"
)

func (a *AIService) Text(stream io.Reader) (text string, err error) {
	req := openai.AudioRequest{
		Model:    whisperModel,
		FilePath: "voice.wav", // Add FilePath here to avoid error
		Reader:   stream,
	}

	resp, err := a.client.CreateTranscription(context.Background(), req)
	if err != nil {
		return "", err
	}

	return resp.Text, nil
}
