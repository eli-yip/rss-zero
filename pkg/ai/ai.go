package ai

import (
	"io"

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

func NewAIService(APIKey string) AIIface {
	if APIKey == "" {
		return &AIServiceWithoutAPI{}
	}
	client := openai.NewClient(APIKey)
	return &AIService{client: client}
}

type AIService struct{ client *openai.Client }

// AIServiceWithoutAPI is a mock service for testing purposes.
// It will be enabled when API key is not provided.
// Polish will return the same text as input.
// Text will return empty string.
type AIServiceWithoutAPI struct{}

func (s *AIServiceWithoutAPI) Polish(text string) (result string, err error) { return text, nil }

func (s *AIServiceWithoutAPI) Text(stream io.Reader) (text string, err error) { return "", nil }
