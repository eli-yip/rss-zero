package ai

import (
	"context"

	"github.com/sashabaranov/go-openai"
)

func (a *AIService) Text(path string) (text string, err error) {
	ctx := context.Background()

	req := openai.AudioRequest{
		Model:    openai.Whisper1,
		FilePath: path,
	}

	resp, err := a.client.CreateTranscription(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Text, nil
}
