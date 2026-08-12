package memory

import "context"

// Embedder generates embedding vectors for texts.
type Embedder interface {
	Embed(ctx context.Context, texts []string) ([][]float64, error)
}

// EmbedderFunc wraps a function as an Embedder.
type EmbedderFunc func(ctx context.Context, texts []string) ([][]float64, error)

func (f EmbedderFunc) Embed(ctx context.Context, texts []string) ([][]float64, error) {
	return f(ctx, texts)
}
