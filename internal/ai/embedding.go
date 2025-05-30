package ai

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

const embeddingModel = "doubao-embedding-large-text-250515"

func (a *AIService) Embed(text string) (result []float32, err error) {
	req := openai.EmbeddingRequestStrings{
		Input:          []string{text},
		Model:          embeddingModel,
		EncodingFormat: openai.EmbeddingEncodingFormatFloat,
	}

	resp, err := a.client.CreateEmbeddings(context.Background(), req)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding: %w", err)
	}
	return resp.Data[0].Embedding, nil
}
