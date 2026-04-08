# ONNX Vector Search Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add semantic vector search to mempalace-go using the same all-MiniLM-L6-v2 ONNX model as ChromaDB, targeting parity with Python's 96.6% R@5.

**Architecture:** ONNX Runtime Go bindings generate 384-dim embeddings via MiniLM. Embeddings stored as BLOBs in a new SQLite table. Search computes cosine similarity in Go (brute-force — fast enough for <100K vectors). WordPiece tokenizer implemented in pure Go from vocab.txt. Mean pooling + L2 normalization matches sentence-transformers output exactly.

**Tech Stack:**
- `github.com/yalue/onnxruntime_go` — Go bindings for ONNX Runtime
- `brew install onnxruntime` — shared library (macOS ARM64)
- Model: `~/.cache/chroma/onnx_models/all-MiniLM-L6-v2/onnx/model.onnx` (already downloaded)
- Tokenizer: WordPiece from `vocab.txt` (30,522 tokens)

---

## Model Details (verified)

- **Inputs:** `input_ids` (int64), `attention_mask` (int64), `token_type_ids` (int64) — all shape `[batch, seq_len]`
- **Output:** `last_hidden_state` — shape `[batch, seq_len, 384]`
- **Pooling:** Mean pooling over token embeddings (mask-aware), then L2 normalize
- **Max sequence length:** 512 tokens
- **Tokenizer:** BERT WordPiece, lowercase, `[CLS]`/`[SEP]` special tokens, vocab size 30,522

---

### Task 1: Install ONNX Runtime + Verify Go Bindings

**Step 1: Install ONNX Runtime**

```bash
brew install onnxruntime
```

**Step 2: Verify library location**

```bash
ls /opt/homebrew/lib/libonnxruntime*
```

**Step 3: Add Go dependency**

```bash
cd ~/work/mempalace-go && go get github.com/yalue/onnxruntime_go
```

**Step 4: Write smoke test**

Create `internal/embed/embed_test.go`:

```go
package embed

import (
	"testing"
)

func TestOnnxRuntimeLoads(t *testing.T) {
	// Just verify ONNX Runtime can be initialized
	if err := InitRuntime(); err != nil {
		t.Skipf("ONNX Runtime not available: %v", err)
	}
	defer DestroyRuntime()
}
```

Create minimal `internal/embed/embed.go`:

```go
package embed

import ort "github.com/yalue/onnxruntime_go"

func InitRuntime() error {
	ort.SetSharedLibraryPath("/opt/homebrew/lib/libonnxruntime.dylib")
	return ort.InitializeEnvironment()
}

func DestroyRuntime() {
	ort.DestroyEnvironment()
}
```

**Step 5: Run test**

```bash
go test ./internal/embed/ -v -run TestOnnxRuntimeLoads
```

**Step 6: Commit**

```bash
git add internal/embed/ go.mod go.sum
git commit -m "feat: add ONNX Runtime Go bindings, verify initialization"
```

---

### Task 2: WordPiece Tokenizer

Implement BERT WordPiece tokenizer in pure Go. Reads `vocab.txt` (one token per line, line number = token ID).

**Files:**
- Create: `internal/embed/tokenizer.go`
- Create: `internal/embed/tokenizer_test.go`

**Step 1: Write failing tests**

```go
package embed

import (
	"os"
	"path/filepath"
	"testing"
)

const vocabPath = "/.cache/chroma/onnx_models/all-MiniLM-L6-v2/onnx/vocab.txt"

func home() string { h, _ := os.UserHomeDir(); return h }

func TestTokenizerLoad(t *testing.T) {
	path := filepath.Join(home(), vocabPath)
	tok, err := NewTokenizer(path)
	if err != nil {
		t.Skipf("vocab.txt not found: %v", err)
	}
	if tok.VocabSize() != 30522 {
		t.Fatalf("expected 30522 vocab, got %d", tok.VocabSize())
	}
}

func TestTokenize(t *testing.T) {
	path := filepath.Join(home(), vocabPath)
	tok, err := NewTokenizer(path)
	if err != nil {
		t.Skip("vocab not available")
	}

	// "this is a test" should tokenize to [CLS]=101, this=2023, is=2003, a=1037, test=3231, [SEP]=102
	ids, mask := tok.Encode("this is a test", 512)
	if len(ids) < 6 {
		t.Fatalf("expected at least 6 tokens, got %d", len(ids))
	}
	if ids[0] != 101 { // [CLS]
		t.Fatalf("expected [CLS]=101, got %d", ids[0])
	}
	if ids[len(ids)-1] != 102 { // [SEP]
		t.Fatalf("expected [SEP]=102, got %d", ids[len(ids)-1])
	}
	if len(ids) != len(mask) {
		t.Fatal("ids and mask length mismatch")
	}
}

func TestTokenizeWordPiece(t *testing.T) {
	path := filepath.Join(home(), vocabPath)
	tok, err := NewTokenizer(path)
	if err != nil {
		t.Skip("vocab not available")
	}

	// "unbelievable" should be split into subwords like "un", "##bel", "##ie", "##va", "##ble"
	ids, _ := tok.Encode("unbelievable", 512)
	// Should have more tokens than just [CLS] + 1 word + [SEP]
	if len(ids) <= 3 {
		t.Fatalf("expected WordPiece splitting, got %d tokens", len(ids))
	}
}
```

**Step 2: Implement tokenizer**

```go
package embed

import (
	"bufio"
	"os"
	"strings"
	"unicode"
)

type Tokenizer struct {
	vocab   map[string]int64
	idToTok []string
}

func NewTokenizer(vocabPath string) (*Tokenizer, error) {
	f, err := os.Open(vocabPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vocab := make(map[string]int64)
	var idToTok []string
	scanner := bufio.NewScanner(f)
	var id int64
	for scanner.Scan() {
		tok := scanner.Text()
		vocab[tok] = id
		idToTok = append(idToTok, tok)
		id++
	}
	return &Tokenizer{vocab: vocab, idToTok: idToTok}, nil
}

func (t *Tokenizer) VocabSize() int { return len(t.vocab) }

func (t *Tokenizer) Encode(text string, maxLen int) (ids []int64, mask []int64) {
	// Lowercase
	text = strings.ToLower(text)

	// Basic tokenization: split on whitespace and punctuation
	words := basicTokenize(text)

	// WordPiece each word
	var tokens []int64
	for _, word := range words {
		tokens = append(tokens, t.wordPiece(word)...)
	}

	// Truncate to maxLen - 2 (leave room for [CLS] and [SEP])
	if len(tokens) > maxLen-2 {
		tokens = tokens[:maxLen-2]
	}

	// Add [CLS] and [SEP]
	clsID := t.vocab["[CLS]"]
	sepID := t.vocab["[SEP]"]
	ids = make([]int64, 0, len(tokens)+2)
	ids = append(ids, clsID)
	ids = append(ids, tokens...)
	ids = append(ids, sepID)

	mask = make([]int64, len(ids))
	for i := range mask {
		mask[i] = 1
	}
	return ids, mask
}

func basicTokenize(text string) []string {
	// Split on whitespace, then separate punctuation
	var tokens []string
	for _, word := range strings.Fields(text) {
		tokens = append(tokens, splitPunctuation(word)...)
	}
	return tokens
}

func splitPunctuation(word string) []string {
	var tokens []string
	var current strings.Builder
	for _, r := range word {
		if unicode.IsPunct(r) || unicode.IsSymbol(r) {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, string(r))
		} else {
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func (t *Tokenizer) wordPiece(word string) []int64 {
	if _, ok := t.vocab[word]; ok {
		return []int64{t.vocab[word]}
	}

	var tokens []int64
	start := 0
	for start < len(word) {
		end := len(word)
		found := false
		for end > start {
			substr := word[start:end]
			if start > 0 {
				substr = "##" + substr
			}
			if id, ok := t.vocab[substr]; ok {
				tokens = append(tokens, id)
				found = true
				start = end
				break
			}
			end--
		}
		if !found {
			// Unknown token
			tokens = append(tokens, t.vocab["[UNK]"])
			start++
		}
	}
	return tokens
}
```

**Step 3: Run tests, verify pass**

**Step 4: Commit**

```bash
git commit -m "feat: WordPiece tokenizer from vocab.txt"
```

---

### Task 3: Embedder — ONNX Inference + Mean Pooling

Generate 384-dim sentence embeddings matching ChromaDB's output exactly.

**Files:**
- Modify: `internal/embed/embed.go`
- Create: `internal/embed/embed_test.go` (add to existing)

**Step 1: Write failing test**

```go
func TestEmbed(t *testing.T) {
	modelDir := filepath.Join(home(), ".cache/chroma/onnx_models/all-MiniLM-L6-v2/onnx")
	emb, err := NewEmbedder(modelDir)
	if err != nil {
		t.Skipf("model not available: %v", err)
	}
	defer emb.Close()

	vec, err := emb.Embed("this is a test")
	if err != nil {
		t.Fatal(err)
	}
	if len(vec) != 384 {
		t.Fatalf("expected 384 dims, got %d", len(vec))
	}

	// Should be L2-normalized (norm ≈ 1.0)
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	if norm < 0.99 || norm > 1.01 {
		t.Fatalf("expected unit norm, got %.4f", norm)
	}
}

func TestEmbedBatch(t *testing.T) {
	modelDir := filepath.Join(home(), ".cache/chroma/onnx_models/all-MiniLM-L6-v2/onnx")
	emb, err := NewEmbedder(modelDir)
	if err != nil {
		t.Skip("model not available")
	}
	defer emb.Close()

	vecs, err := emb.EmbedBatch([]string{"hello world", "goodbye world"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 2 {
		t.Fatalf("expected 2 vectors, got %d", len(vecs))
	}
}

func TestEmbedSimilarity(t *testing.T) {
	modelDir := filepath.Join(home(), ".cache/chroma/onnx_models/all-MiniLM-L6-v2/onnx")
	emb, err := NewEmbedder(modelDir)
	if err != nil {
		t.Skip("model not available")
	}
	defer emb.Close()

	v1, _ := emb.Embed("the cat sat on the mat")
	v2, _ := emb.Embed("a kitten was sitting on a rug")
	v3, _ := emb.Embed("database migration to PostgreSQL")

	sim12 := CosineSim(v1, v2)
	sim13 := CosineSim(v1, v3)

	// Similar sentences should have higher similarity
	if sim12 <= sim13 {
		t.Fatalf("expected sim(cat,kitten)=%.3f > sim(cat,database)=%.3f", sim12, sim13)
	}
}
```

**Step 2: Implement Embedder**

```go
package embed

import (
	"fmt"
	"math"
	"path/filepath"

	ort "github.com/yalue/onnxruntime_go"
)

const embDim = 384

type Embedder struct {
	session   *ort.DynamicAdvancedSession
	tokenizer *Tokenizer
}

func NewEmbedder(modelDir string) (*Embedder, error) {
	if err := InitRuntime(); err != nil {
		return nil, err
	}

	tok, err := NewTokenizer(filepath.Join(modelDir, "vocab.txt"))
	if err != nil {
		return nil, fmt.Errorf("load tokenizer: %w", err)
	}

	modelPath := filepath.Join(modelDir, "model.onnx")
	inputs := []string{"input_ids", "attention_mask", "token_type_ids"}
	outputs := []string{"last_hidden_state"}
	session, err := ort.NewDynamicAdvancedSession(modelPath, inputs, outputs, nil)
	if err != nil {
		return nil, fmt.Errorf("load model: %w", err)
	}

	return &Embedder{session: session, tokenizer: tok}, nil
}

func (e *Embedder) Close() {
	if e.session != nil {
		e.session.Destroy()
	}
	DestroyRuntime()
}

func (e *Embedder) Embed(text string) ([]float32, error) {
	vecs, err := e.EmbedBatch([]string{text})
	if err != nil {
		return nil, err
	}
	return vecs[0], nil
}

func (e *Embedder) EmbedBatch(texts []string) ([][]float32, error) {
	batch := len(texts)
	// Tokenize all texts, find max length
	allIDs := make([][]int64, batch)
	allMasks := make([][]int64, batch)
	maxLen := 0
	for i, text := range texts {
		ids, mask := e.tokenizer.Encode(text, 512)
		allIDs[i] = ids
		allMasks[i] = mask
		if len(ids) > maxLen {
			maxLen = len(ids)
		}
	}

	// Pad to maxLen and flatten for ONNX
	flatIDs := make([]int64, batch*maxLen)
	flatMask := make([]int64, batch*maxLen)
	flatType := make([]int64, batch*maxLen) // all zeros
	for i := 0; i < batch; i++ {
		for j := 0; j < len(allIDs[i]); j++ {
			flatIDs[i*maxLen+j] = allIDs[i][j]
			flatMask[i*maxLen+j] = allMasks[i][j]
		}
		// Rest stays 0 (padding)
	}

	// Create tensors
	shape := ort.Shape{int64(batch), int64(maxLen)}
	inputIDs, _ := ort.NewTensor(shape, flatIDs)
	defer inputIDs.Destroy()
	attnMask, _ := ort.NewTensor(shape, flatMask)
	defer attnMask.Destroy()
	tokenType, _ := ort.NewTensor(shape, flatType)
	defer tokenType.Destroy()

	// Output tensor
	outShape := ort.Shape{int64(batch), int64(maxLen), embDim}
	output, _ := ort.NewEmptyTensor[float32](outShape)
	defer output.Destroy()

	// Run inference
	err := e.session.Run([]ort.ArbitraryTensor{inputIDs, attnMask, tokenType},
		[]ort.ArbitraryTensor{output})
	if err != nil {
		return nil, fmt.Errorf("inference: %w", err)
	}

	// Mean pooling + L2 normalize
	hidden := output.GetData()
	results := make([][]float32, batch)
	for i := 0; i < batch; i++ {
		vec := make([]float32, embDim)
		seqLen := len(allMasks[i]) // actual length before padding
		for j := 0; j < seqLen; j++ {
			if allMasks[i][j] == 1 {
				offset := (i*maxLen + j) * embDim
				for k := 0; k < embDim; k++ {
					vec[k] += hidden[offset+k]
				}
			}
		}
		// Divide by number of active tokens
		count := float32(0)
		for _, m := range allMasks[i] {
			count += float32(m)
		}
		if count > 0 {
			for k := range vec {
				vec[k] /= count
			}
		}
		// L2 normalize
		var norm float64
		for _, v := range vec {
			norm += float64(v) * float64(v)
		}
		norm = math.Sqrt(norm)
		if norm > 0 {
			for k := range vec {
				vec[k] = float32(float64(vec[k]) / norm)
			}
		}
		results[i] = vec
	}
	return results, nil
}

func CosineSim(a, b []float32) float64 {
	var dot float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
	}
	return dot // both are L2-normalized, so dot = cosine
}
```

**Step 3: Run tests**

**Step 4: Commit**

```bash
git commit -m "feat: ONNX embedder — MiniLM-L6-v2 inference with mean pooling"
```

---

### Task 4: Vector Storage in SQLite

Add an embeddings table to the store and a vector search method.

**Files:**
- Modify: `internal/store/store.go`
- Modify: `internal/store/store_test.go`

**Step 1: Write failing tests**

```go
func TestAddWithEmbedding(t *testing.T) {
	s := tempDB(t)
	vec := make([]float32, 384)
	vec[0] = 1.0 // unit vector along first dim

	err := s.AddWithEmbedding(Drawer{ID: "d1", Document: "test", Wing: "w", Room: "r"}, vec)
	if err != nil {
		t.Fatal(err)
	}

	// Verify it's stored
	n, _ := s.Count()
	if n != 1 {
		t.Fatalf("expected 1, got %d", n)
	}
}

func TestVectorSearch(t *testing.T) {
	s := tempDB(t)

	// Create 3 vectors: v1 and v2 similar, v3 different
	v1 := make([]float32, 384)
	v2 := make([]float32, 384)
	v3 := make([]float32, 384)
	v1[0], v1[1] = 0.9, 0.1
	v2[0], v2[1] = 0.8, 0.2
	v3[0], v3[1] = 0.1, 0.9

	s.AddWithEmbedding(Drawer{ID: "d1", Document: "similar A", Wing: "w", Room: "r"}, v1)
	s.AddWithEmbedding(Drawer{ID: "d2", Document: "similar B", Wing: "w", Room: "r"}, v2)
	s.AddWithEmbedding(Drawer{ID: "d3", Document: "different", Wing: "w", Room: "r"}, v3)

	query := make([]float32, 384)
	query[0] = 1.0 // close to v1 and v2

	results, err := s.VectorSearch(query, 2, Query{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// d1 should be first (closest)
	if results[0].ID != "d1" {
		t.Fatalf("expected d1 first, got %s", results[0].ID)
	}
}
```

**Step 2: Implement**

Add to `store.go`:
- New table: `CREATE TABLE IF NOT EXISTS embeddings (drawer_id TEXT PRIMARY KEY, vector BLOB, FOREIGN KEY(drawer_id) REFERENCES drawers(id))`
- `AddWithEmbedding(d Drawer, vec []float32)` — insert drawer + embedding
- `VectorSearch(query []float32, limit int, q Query) ([]SearchResult, error)` — brute-force cosine similarity

Embedding storage: 384 float32 = 1536 bytes per vector as raw BLOB.

```go
func (s *Store) AddWithEmbedding(d Drawer, vec []float32) error {
	if err := s.Add(d); err != nil {
		return err
	}
	blob := float32ToBytes(vec)
	_, err := s.db.Exec("INSERT OR REPLACE INTO embeddings (drawer_id, vector) VALUES (?, ?)", d.ID, blob)
	return err
}

func (s *Store) VectorSearch(query []float32, limit int, q Query) ([]SearchResult, error) {
	// Get all embeddings (with optional metadata filters)
	where, args := q.buildWhere()
	sql := `SELECT d.id, d.document, d.wing, d.room, d.source, d.filed_at, d.hall, e.vector
		FROM embeddings e JOIN drawers d ON d.id = e.drawer_id`
	if where != "" {
		sql += " WHERE " + where
	}

	rows, err := s.db.Query(sql, args...)
	// ... compute cosine similarity for each row against query
	// ... sort by similarity descending, return top-limit
}
```

**Step 3: Run tests, verify pass**

**Step 4: Commit**

```bash
git commit -m "feat: vector storage and cosine similarity search in SQLite"
```

---

### Task 5: Update Benchmark for Vector Search

Add `--mode vector` to the bench runner.

**Files:**
- Modify: `cmd/bench/main.go`

**Step 1: Add vector mode**

The vector mode benchmark flow per question:
1. Create fresh store
2. Initialize embedder (reuse across questions for speed)
3. Embed all haystack sessions, store with AddWithEmbedding
4. Embed query, run VectorSearch
5. Check if gold session in top-5/10

Key change: embedder is initialized once, outside the question loop.

```go
// In main():
var emb *embed.Embedder
if *mode == "vector" {
	modelDir := filepath.Join(home(), ".cache/chroma/onnx_models/all-MiniLM-L6-v2/onnx")
	emb, err = embed.NewEmbedder(modelDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "embedder init error:", err)
		os.Exit(1)
	}
	defer emb.Close()
}

// In question loop, if mode == "vector":
// 1. Embed all sessions
// 2. Store with AddWithEmbedding
// 3. Embed query
// 4. VectorSearch
```

**Step 2: Run benchmark**

```bash
./bin/bench --data /tmp/longmemeval.json --limit 50 --mode vector --csv /tmp/go_vector_50.csv
```

**Step 3: Commit**

```bash
git commit -m "feat: vector search mode in LongMemEval benchmark"
```

---

### Task 6: Full 500-Question Benchmark + Results

**Step 1: Run all 3 modes**

```bash
./bin/bench --data /tmp/longmemeval.json --limit 500 --mode raw --csv /tmp/go_raw_500.csv
./bin/bench --data /tmp/longmemeval.json --limit 500 --mode vector --csv /tmp/go_vector_500.csv
```

**Step 2: Compare and update README with results table**

**Step 3: Commit**

```bash
git commit -m "docs: benchmark results — raw vs hybrid vs vector vs Python"
```

---

## Summary

| Task | What | Effort |
|------|------|--------|
| 1 | ONNX Runtime install + Go bindings | 10 min |
| 2 | WordPiece tokenizer | 30 min |
| 3 | Embedder (inference + pooling) | 30 min |
| 4 | Vector storage + cosine search | 20 min |
| 5 | Benchmark vector mode | 15 min |
| 6 | Full benchmark + results | 15 min |
