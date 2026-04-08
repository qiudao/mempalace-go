package embed

import (
	"math"
	"os"
	"testing"
)

const modelDir = "/Users/k/.cache/chroma/onnx_models/all-MiniLM-L6-v2/onnx"

func vocabPath() string {
	return modelDir + "/vocab.txt"
}

func skipIfNoModel(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(modelDir + "/model.onnx"); err != nil {
		t.Skip("ONNX model not available, skipping")
	}
	if _, err := os.Stat(modelDir + "/vocab.txt"); err != nil {
		t.Skip("vocab.txt not available, skipping")
	}
}

func TestTokenizerLoad(t *testing.T) {
	skipIfNoModel(t)
	tok, err := NewTokenizer(vocabPath())
	if err != nil {
		t.Fatal(err)
	}
	if tok.VocabSize() != 30522 {
		t.Fatalf("expected vocab size 30522, got %d", tok.VocabSize())
	}
}

func TestTokenize(t *testing.T) {
	skipIfNoModel(t)
	tok, err := NewTokenizer(vocabPath())
	if err != nil {
		t.Fatal(err)
	}
	ids, mask := tok.Encode("this is a test", 128)

	// Should start with [CLS]=101
	if ids[0] != 101 {
		t.Fatalf("expected first token 101 ([CLS]), got %d", ids[0])
	}

	// Find [SEP]=102
	var sepIdx int
	for i, id := range ids {
		if id == 102 {
			sepIdx = i
			break
		}
	}
	if sepIdx == 0 {
		t.Fatal("did not find [SEP] token")
	}

	// Attention mask should be 1 up to and including [SEP], 0 after
	for i := 0; i <= sepIdx; i++ {
		if mask[i] != 1 {
			t.Fatalf("expected mask[%d]=1, got %d", i, mask[i])
		}
	}
	for i := sepIdx + 1; i < len(mask); i++ {
		if mask[i] != 0 {
			t.Fatalf("expected mask[%d]=0, got %d", i, mask[i])
		}
	}

	t.Logf("Tokenized 'this is a test' -> ids[0:%d]: %v", sepIdx+1, ids[:sepIdx+1])
}

var runtimeInitialized bool

func setupRuntime(t *testing.T) {
	t.Helper()
	if !runtimeInitialized {
		if err := InitRuntime(); err != nil {
			t.Fatalf("InitRuntime: %v", err)
		}
		runtimeInitialized = true
	}
}

func TestEmbed(t *testing.T) {
	skipIfNoModel(t)
	setupRuntime(t)

	emb, err := NewEmbedder(modelDir)
	if err != nil {
		t.Fatal(err)
	}
	defer emb.Close()

	vec, err := emb.Embed("this is a test")
	if err != nil {
		t.Fatal(err)
	}

	if len(vec) != 384 {
		t.Fatalf("expected 384-dim vector, got %d", len(vec))
	}

	// Check it's a unit vector (L2 norm ~= 1.0)
	var norm float64
	for _, v := range vec {
		norm += float64(v) * float64(v)
	}
	norm = math.Sqrt(norm)
	if math.Abs(norm-1.0) > 0.01 {
		t.Fatalf("expected unit vector (norm=1.0), got norm=%f", norm)
	}
}

func TestEmbedSimilarity(t *testing.T) {
	skipIfNoModel(t)
	setupRuntime(t)

	emb, err := NewEmbedder(modelDir)
	if err != nil {
		t.Fatal(err)
	}
	defer emb.Close()

	vecs, err := emb.EmbedBatch([]string{
		"the cat sat on the mat",
		"a kitten was sitting on a rug",
		"database migration to PostgreSQL",
	})
	if err != nil {
		t.Fatal(err)
	}

	simSimilar := CosineSim(vecs[0], vecs[1])
	simDifferent := CosineSim(vecs[0], vecs[2])

	t.Logf("cat/kitten similarity: %.4f", simSimilar)
	t.Logf("cat/database similarity: %.4f", simDifferent)

	if simSimilar <= simDifferent {
		t.Fatalf("expected similar sentences (%.4f) > dissimilar (%.4f)", simSimilar, simDifferent)
	}
}
