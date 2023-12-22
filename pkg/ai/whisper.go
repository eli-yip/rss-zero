package ai

import (
	"context"
	"io"

	"github.com/sashabaranov/go-openai"
)

func (a *AIService) Text(stream io.Reader) (text string, err error) {
	ctx := context.Background()

	req := openai.AudioRequest{
		Model:  openai.Whisper1,
		Reader: stream,
	}

	resp, err := a.client.CreateTranscription(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Text, nil
}
