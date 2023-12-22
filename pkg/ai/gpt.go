package ai

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

func (a *AIService) Polish(text string) (result string, err error) {
	ctx := context.Background()

	req := openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo1106,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("请为我格式化下面的文本，使其通顺完整，谢谢！\n\"\"\"%s\"\"\"", text),
			},
		},
	}

	resp, err := a.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}
