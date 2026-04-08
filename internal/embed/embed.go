package embed

import (
	"fmt"
	"math"
	"path/filepath"

	ort "github.com/yalue/onnxruntime_go"
)

const defaultMaxLen = 256

var runtimeInitialized bool

// InitRuntime initialises the ONNX Runtime shared library.
func InitRuntime() error {
	if runtimeInitialized {
		return nil
	}
	ort.SetSharedLibraryPath("/opt/homebrew/lib/libonnxruntime.dylib")
	if err := ort.InitializeEnvironment(); err != nil {
		return err
	}
	runtimeInitialized = true
	return nil
}

// DestroyRuntime tears down the ONNX Runtime environment.
func DestroyRuntime() {
	ort.DestroyEnvironment()
}

// Embedder generates 384-dimensional sentence embeddings using an ONNX model.
type Embedder struct {
	session   *ort.DynamicAdvancedSession
	tokenizer *Tokenizer
}

// NewEmbedder loads the ONNX model and vocab from modelDir.
func NewEmbedder(modelDir string) (*Embedder, error) {
	if err := InitRuntime(); err != nil {
		return nil, fmt.Errorf("init onnx runtime: %w", err)
	}

	vocabPath := filepath.Join(modelDir, "vocab.txt")
	tok, err := NewTokenizer(vocabPath)
	if err != nil {
		return nil, fmt.Errorf("load tokenizer: %w", err)
	}

	modelPath := filepath.Join(modelDir, "model.onnx")
	inputNames := []string{"input_ids", "attention_mask", "token_type_ids"}
	outputNames := []string{"last_hidden_state"}

	session, err := ort.NewDynamicAdvancedSession(modelPath, inputNames, outputNames, nil)
	if err != nil {
		return nil, fmt.Errorf("create onnx session: %w", err)
	}

	return &Embedder{session: session, tokenizer: tok}, nil
}

// Close releases the ONNX session resources.
func (e *Embedder) Close() {
	if e.session != nil {
		e.session.Destroy()
	}
}

// Embed generates a 384-dim embedding for a single text.
func (e *Embedder) Embed(text string) ([]float32, error) {
	results, err := e.EmbedBatch([]string{text})
	if err != nil {
		return nil, err
	}
	return results[0], nil
}

// EmbedBatch generates embeddings for multiple texts.
// Large batches are split into sub-batches of 8 to limit memory usage.
func (e *Embedder) EmbedBatch(texts []string) ([][]float32, error) {
	const maxBatch = 8
	if len(texts) > maxBatch {
		var all [][]float32
		for i := 0; i < len(texts); i += maxBatch {
			end := i + maxBatch
			if end > len(texts) {
				end = len(texts)
			}
			vecs, err := e.embedBatchInner(texts[i:end])
			if err != nil {
				return nil, err
			}
			all = append(all, vecs...)
		}
		return all, nil
	}
	return e.embedBatchInner(texts)
}

func (e *Embedder) embedBatchInner(texts []string) ([][]float32, error) {
	batch := int64(len(texts))
	seqLen := int64(defaultMaxLen)
	shape := ort.Shape{batch, seqLen}

	// Collect all token data
	allIDs := make([]int64, 0, batch*seqLen)
	allMask := make([]int64, 0, batch*seqLen)
	allTypeIDs := make([]int64, batch*seqLen) // all zeros

	for _, text := range texts {
		ids, mask := e.tokenizer.Encode(text, defaultMaxLen)
		allIDs = append(allIDs, ids...)
		allMask = append(allMask, mask...)
	}

	inputIDs, err := ort.NewTensor(shape, allIDs)
	if err != nil {
		return nil, fmt.Errorf("create input_ids tensor: %w", err)
	}
	defer inputIDs.Destroy()

	attMask, err := ort.NewTensor(shape, allMask)
	if err != nil {
		return nil, fmt.Errorf("create attention_mask tensor: %w", err)
	}
	defer attMask.Destroy()

	typeIDs, err := ort.NewTensor(shape, allTypeIDs)
	if err != nil {
		return nil, fmt.Errorf("create token_type_ids tensor: %w", err)
	}
	defer typeIDs.Destroy()

	outShape := ort.Shape{batch, seqLen, 384}
	output, err := ort.NewEmptyTensor[float32](outShape)
	if err != nil {
		return nil, fmt.Errorf("create output tensor: %w", err)
	}
	defer output.Destroy()

	err = e.session.Run(
		[]ort.ArbitraryTensor{inputIDs, attMask, typeIDs},
		[]ort.ArbitraryTensor{output},
	)
	if err != nil {
		return nil, fmt.Errorf("onnx run: %w", err)
	}

	hidden := output.GetData()
	results := make([][]float32, batch)

	for i := int64(0); i < batch; i++ {
		// Mean pooling with attention mask
		embedding := make([]float32, 384)
		var maskSum float32
		for j := int64(0); j < seqLen; j++ {
			m := float32(allMask[i*seqLen+j])
			if m == 0 {
				continue
			}
			maskSum += m
			offset := i*seqLen*384 + j*384
			for k := int64(0); k < 384; k++ {
				embedding[k] += hidden[offset+k] * m
			}
		}
		if maskSum > 0 {
			for k := range embedding {
				embedding[k] /= maskSum
			}
		}

		// L2 normalize
		var norm float64
		for _, v := range embedding {
			norm += float64(v) * float64(v)
		}
		norm = math.Sqrt(norm)
		if norm > 0 {
			for k := range embedding {
				embedding[k] = float32(float64(embedding[k]) / norm)
			}
		}

		results[i] = embedding
	}

	return results, nil
}

// CosineSim computes the cosine similarity between two vectors.
func CosineSim(a, b []float32) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}
