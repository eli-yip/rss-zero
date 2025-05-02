package ai

import (
	"io"
	"net/url"

	openai "github.com/sashabaranov/go-openai"
)

// whisperModel is the whisper model used for speech to text.
//
// It should be changed as openai updates their models.
const whisperModel = openai.Whisper1

// AI interface is for AI related services,
// such as text generation based on gpt-3.5 model,
// or speech to text based on whisper model.
type AI interface {
	// Polish method will take a text and return a polished version of it.
	Polish(text string) (result string, err error)
	// Text method will take a io.Reader and return the transcribed text.
	Text(path io.Reader) (text string, err error)
	// Conclude method will take a text and return the conclusion of it.
	Conclude(text string) (result string, err error)
	TranslateToZh(text string) (result string, err error)
}

// AIService implements AI interface.
// It uses openai client to communicate with openai API,
// and provides Polish and Text methods.
type AIService struct{ client *openai.Client }

// When API Key is not provided, it will use AIServiceWithoutAPI,
// which is a mock service for testing purposes.
func NewAIService(apiKey string, baseURL string) AI {
	if apiKey == "" {
		return &AIServiceWithoutAPI{}
	}

	clientConfig := openai.DefaultConfig(apiKey)
	url, _ := url.Parse(baseURL)
	clientConfig.BaseURL = url.String()
	return &AIService{client: openai.NewClientWithConfig(clientConfig)}
}

// AIServiceWithoutAPI is a mock service for testing purposes.
// It will be enabled when API key is not provided.
// Polish will return the same text as input.
// Text will return empty string.
type AIServiceWithoutAPI struct{}

func (s *AIServiceWithoutAPI) Polish(text string) (result string, err error) { return text, nil }

func (s *AIServiceWithoutAPI) Text(stream io.Reader) (text string, err error) { return "", nil }

func (s *AIServiceWithoutAPI) Conclude(text string) (result string, err error) { return text, nil }

func (s *AIServiceWithoutAPI) TranslateToZh(text string) (result string, err error) { return text, nil }
