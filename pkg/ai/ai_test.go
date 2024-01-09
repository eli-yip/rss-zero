package ai

import (
	"fmt"
	"os"
	"testing"
)

func TestBaseURL(t *testing.T) {
	apiKey := os.Getenv("API_KEY")
	baseURL := os.Getenv("BASE_URL")
	fmt.Println(apiKey, baseURL)
	ai := NewAIService(apiKey, baseURL)
	result, err := ai.Polish("test")
	if err != nil {
		t.Error(err)
	}
	fmt.Println(result)
}
