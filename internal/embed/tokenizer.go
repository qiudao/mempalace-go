package embed

import (
	"bufio"
	"os"
	"strings"
	"unicode"
)

// Tokenizer implements a BERT WordPiece tokenizer.
type Tokenizer struct {
	vocab map[string]int64
}

const (
	clsID = int64(101)
	sepID = int64(102)
	unkID = int64(100)
)

// NewTokenizer loads a vocab.txt file and returns a Tokenizer.
func NewTokenizer(vocabPath string) (*Tokenizer, error) {
	f, err := os.Open(vocabPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	vocab := make(map[string]int64)
	scanner := bufio.NewScanner(f)
	var idx int64
	for scanner.Scan() {
		vocab[scanner.Text()] = idx
		idx++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return &Tokenizer{vocab: vocab}, nil
}

// VocabSize returns the number of tokens in the vocabulary.
func (t *Tokenizer) VocabSize() int {
	return len(t.vocab)
}

// Encode tokenizes text into input_ids and attention_mask, padded/truncated to maxLen.
func (t *Tokenizer) Encode(text string, maxLen int) (ids []int64, mask []int64) {
	tokens := t.tokenize(text)

	// Reserve space for [CLS] and [SEP]
	if len(tokens) > maxLen-2 {
		tokens = tokens[:maxLen-2]
	}

	ids = make([]int64, maxLen)
	mask = make([]int64, maxLen)

	ids[0] = clsID
	mask[0] = 1
	for i, tok := range tokens {
		ids[i+1] = tok
		mask[i+1] = 1
	}
	ids[len(tokens)+1] = sepID
	mask[len(tokens)+1] = 1
	// remaining positions stay 0 (padding)

	return ids, mask
}

// tokenize splits text into WordPiece token IDs.
func (t *Tokenizer) tokenize(text string) []int64 {
	text = strings.ToLower(text)
	words := splitOnWhitespaceAndPunctuation(text)

	var ids []int64
	for _, word := range words {
		ids = append(ids, t.wordPiece(word)...)
	}
	return ids
}

// splitOnWhitespaceAndPunctuation splits text into words, treating each
// punctuation character as its own token.
func splitOnWhitespaceAndPunctuation(text string) []string {
	var tokens []string
	var current strings.Builder
	for _, r := range text {
		if unicode.IsSpace(r) {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		} else if unicode.IsPunct(r) || unicode.IsSymbol(r) {
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

// wordPiece applies the WordPiece algorithm to a single word.
func (t *Tokenizer) wordPiece(word string) []int64 {
	if _, ok := t.vocab[word]; ok {
		return []int64{t.vocab[word]}
	}

	var ids []int64
	start := 0
	for start < len(word) {
		end := len(word)
		var found bool
		for end > start {
			substr := word[start:end]
			if start > 0 {
				substr = "##" + substr
			}
			if id, ok := t.vocab[substr]; ok {
				ids = append(ids, id)
				found = true
				break
			}
			end--
		}
		if !found {
			return []int64{unkID}
		}
		start = end
	}
	return ids
}
