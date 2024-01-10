package ai

import (
	"io"
	"net/url"

	openai "github.com/sashabaranov/go-openai"
)

// AIIface interface is for AIIface related services,
// such as text generation based on gpt-3.5 model,
// or speech to text based on whisper model.
// It must implement Polish and Text methods.
// Polish method is for text generation,
// It takes text as input and returns polished text as output.
// Text method is for speech to text,
// It takes path to audio file as input and returns text as output.
type AIIface interface {
	Polish(text string) (result string, err error)
	Text(path io.Reader) (text string, err error)
}

type AIService struct{ client *openai.Client }

func NewAIService(apiKey string, baseURL string) AIIface {
	if apiKey == "" {
		return &AIServiceWithoutAPI{}
	}
	clientConfig := openai.DefaultConfig(apiKey)
	url, _ := url.Parse(baseURL)
	clientConfig.BaseURL = url.String()
	client := openai.NewClientWithConfig(clientConfig)
	return &AIService{client: client}
}

// AIServiceWithoutAPI is a mock service for testing purposes.
// It will be enabled when API key is not provided.
// Polish will return the same text as input.
// Text will return empty string.
type AIServiceWithoutAPI struct{}

func (s *AIServiceWithoutAPI) Polish(text string) (result string, err error) { return text, nil }

func (s *AIServiceWithoutAPI) Text(stream io.Reader) (text string, err error) { return "", nil }
