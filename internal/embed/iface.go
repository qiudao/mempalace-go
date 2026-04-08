package embed

// EmbedderI is the interface for all embedding backends (Ollama, ONNX, etc.)
type EmbedderI interface {
	Embed(text string) ([]float32, error)
	EmbedBatch(texts []string) ([][]float32, error)
	Close()
}
