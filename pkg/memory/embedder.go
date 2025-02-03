package memory

// SimpleEmbedder provides basic local embeddings without external API
type SimpleEmbedder struct {
	dimension int
}

// NewSimpleEmbedder creates a new SimpleEmbedder instance
func NewSimpleEmbedder(dimension int) *SimpleEmbedder {
	return &SimpleEmbedder{dimension: dimension}
}

// Embed generates a simple deterministic vector based on the text
func (e *SimpleEmbedder) Embed(text string) ([]float32, error) {
	// Create a simple deterministic vector based on the text
	vector := make([]float32, e.dimension)
	for i := 0; i < e.dimension && i < len(text); i++ {
		// Use character codes to generate vector components
		if i < len(text) {
			vector[i] = float32(text[i]) / 255.0 // Normalize to [0,1]
		}
	}
	return vector, nil
}

// Dimension returns the dimension of the embeddings
func (e *SimpleEmbedder) Dimension() int {
	return e.dimension
}
