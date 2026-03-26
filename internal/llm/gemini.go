package llm

import (
	"context"
	"fmt"
	"os"
)

type GeminiBackend struct {
	model     string
	apiKeyEnv string
}

func NewGemini(model, apiKeyEnv string) *GeminiBackend {
	return &GeminiBackend{
		model:     model,
		apiKeyEnv: apiKeyEnv,
	}
}

func (g *GeminiBackend) Name() string {
	return "gemini/" + g.model
}

func (g *GeminiBackend) IsAvailable(_ context.Context) bool {
	return os.Getenv(g.apiKeyEnv) != ""
}

func (g *GeminiBackend) Chat(_ context.Context, _ []Message, _ []ToolDefinition) (*Response, error) {
	return nil, fmt.Errorf("gemini backend not yet implemented")
}
