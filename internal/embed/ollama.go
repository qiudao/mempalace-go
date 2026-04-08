package embed

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OllamaEmbedder generates embeddings via Ollama's local HTTP API.
// Supports any model: nomic-embed-text, mxbai-embed-large, bge-m3, etc.
type OllamaEmbedder struct {
	baseURL string
	model   string
	dims    int
}

type ollamaRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type ollamaResponse struct {
	Embeddings [][]float32 `json:"embeddings"`
}

// NewOllamaEmbedder creates an embedder using Ollama's local API.
// Performs a test call to verify connectivity and detect dimensions.
func NewOllamaEmbedder(baseURL, model string) (*OllamaEmbedder, error) {
	if baseURL == "" {
		baseURL = "http://localhost:11434"
	}
	if model == "" {
		model = "nomic-embed-text"
	}
	e := &OllamaEmbedder{baseURL: baseURL, model: model}

	// Test call to detect dims
	vecs, err := e.EmbedBatch([]string{"test"})
	if err != nil {
		return nil, fmt.Errorf("ollama test call failed: %w", err)
	}
	e.dims = len(vecs[0])
	return e, nil
}

func (e *OllamaEmbedder) Dims() int { return e.dims }

func (e *OllamaEmbedder) Close() {}

func (e *OllamaEmbedder) Embed(text string) ([]float32, error) {
	vecs, err := e.EmbedBatch([]string{text})
	if err != nil {
		return nil, err
	}
	return vecs[0], nil
}

func (e *OllamaEmbedder) EmbedBatch(texts []string) ([][]float32, error) {
	body, err := json.Marshal(ollamaRequest{Model: e.model, Input: texts})
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(e.baseURL+"/api/embed", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama error %d: %s", resp.StatusCode, string(b))
	}

	var result ollamaResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(result.Embeddings) != len(texts) {
		return nil, fmt.Errorf("expected %d embeddings, got %d", len(texts), len(result.Embeddings))
	}
	return result.Embeddings, nil
}
