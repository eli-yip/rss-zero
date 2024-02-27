package ai

import (
	"context"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
)

func (a *AIService) Polish(text string) (result string, err error) {
	const polishPrompt = "请为我格式化下面的文本，使其通顺完整，谢谢！请使用Markdown格式，并且只需要回答格式化后的文本，不需要其他内容。\n\"\"\"%s\"\"\""

	return a.askGPT(fmt.Sprintf(polishPrompt, text))
}

func (a *AIService) Conclude(text string) (result string, err error) {
	const concludePrompt = "请为下面的内容取一个贴近内容的标题，只需要回答标题的纯文本，不需要其他内容和任何格式。\n\"\"\"%s\"\"\""

	return a.askGPT(fmt.Sprintf(concludePrompt, text))
}

// askGPT will ask model to generate a reply based on the prompt.
func (a *AIService) askGPT(prompt string) (reply string, err error) {
	req := openai.ChatCompletionRequest{
		Model: gptModel,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser,
				Content: prompt}},
	}

	resp, err := a.client.CreateChatCompletion(context.Background(), req)
	if err != nil {
		return "", err
	}

	return resp.Choices[0].Message.Content, nil
}
