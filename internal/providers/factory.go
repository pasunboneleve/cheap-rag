package providers

import (
	"fmt"

	"github.com/dmvianna/cheap-rag/internal/llm"
	"github.com/dmvianna/cheap-rag/internal/providers/anthropic"
	"github.com/dmvianna/cheap-rag/internal/providers/gemini"
	"github.com/dmvianna/cheap-rag/internal/providers/openai"
	"github.com/dmvianna/cheap-rag/internal/providers/xai"
)

func NewEmbeddings(name string) (llm.EmbeddingsProvider, error) {
	switch name {
	case "openai", "openai-compatible":
		return openai.NewClient(), nil
	case "gemini":
		return gemini.NewClient(), nil
	case "xai":
		return xai.NewClient(), nil
	case "anthropic":
		return nil, fmt.Errorf("unsupported embeddings provider: %s (Anthropic is generation-only in this prototype)", name)
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
	case "anthropic":
		return anthropic.NewClient(), nil
	default:
		return nil, fmt.Errorf("unsupported generation provider: %s", name)
	}
}
