package providers

import (
	"fmt"

	"github.com/dmvianna/chatbot-prototype/internal/llm"
	"github.com/dmvianna/chatbot-prototype/internal/providers/gemini"
	"github.com/dmvianna/chatbot-prototype/internal/providers/openai"
	"github.com/dmvianna/chatbot-prototype/internal/providers/xai"
)

func NewEmbeddings(name string) (llm.EmbeddingsProvider, error) {
	switch name {
	case "openai", "openai-compatible":
		return openai.NewClient(), nil
	case "gemini":
		return gemini.NewClient(), nil
	case "xai":
		return xai.NewClient(), nil
	default:
		return nil, fmt.Errorf("unsupported embeddings provider: %s", name)
	}
}

func NewGenerator(name string) (llm.GenerationProvider, error) {
	switch name {
	case "openai", "openai-compatible":
		return openai.NewClient(), nil
	case "gemini":
		return gemini.NewClient(), nil
	case "xai":
		return xai.NewClient(), nil
	default:
		return nil, fmt.Errorf("unsupported generation provider: %s", name)
	}
}
