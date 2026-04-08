// Package dialect implements the AAAK compressed symbolic memory language.
// It achieves ~30x compression for AI memory by replacing entity names with
// short codes, extracting top topics from word frequency, selecting key
// sentences by scoring, detecting emotions via keyword matching, and
// detecting importance flags.
package dialect

import (
	"regexp"
	"sort"
	"strings"
)

// Dialect encodes text into the AAAK compressed symbolic format.
type Dialect struct {
	EntityCodes map[string]string // "Alice" -> "A1"
}

// CompressResult holds the output of a Compress call.
type CompressResult struct {
	Text      string
	Topics    []string
	Emotions  []string
	Flags     []string
	InputLen  int
	OutputLen int
	Ratio     float64
}

// New creates a Dialect with the given entity code mappings.
// If entityCodes is nil, an empty map is used.
func New(entityCodes map[string]string) *Dialect {
	codes := make(map[string]string)
	if entityCodes != nil {
		for name, code := range entityCodes {
			codes[name] = code
			codes[strings.ToLower(name)] = code
		}
	}
	return &Dialect{EntityCodes: codes}
}

// emotionSignals maps keywords found in text to emotion codes.
var emotionSignals = map[string]string{
	"decided":      "determ",
	"prefer":       "convict",
	"worried":      "anx",
	"excited":      "excite",
	"frustrated":   "frust",
	"confused":     "confuse",
	"love":         "love",
	"hate":         "rage",
	"hope":         "hope",
	"fear":         "fear",
	"trust":        "trust",
	"happy":        "joy",
	"sad":          "grief",
	"surprised":    "surprise",
	"grateful":     "grat",
	"curious":      "curious",
	"wonder":       "wonder",
	"anxious":      "anx",
	"relieved":     "relief",
	"satisf":       "satis",
	"disappoint":   "grief",
	"concern":      "anx",
	"hopeless":     "despair",
	"stuck":        "frust",
	"desperate":    "despair",
	"peaceful":     "peace",
	"calm":         "peace",
	"angry":        "rage",
	"terrified":    "fear",
	"joyful":       "joy",
	"vulnerable":   "vul",
	"exhausted":    "exhaust",
	"determined":   "determ",
	"passionate":   "passion",
	"tender":       "tender",
	"humor":        "humor",
	"funny":        "humor",
	"doubt":        "doubt",
	"frustration":  "frust",
	"anxiety":      "anx",
	"excitement":   "excite",
	"satisfaction":  "satis",
	"disappointment":"grief",
}

// flagSignals maps keywords to importance flag types.
var flagSignals = map[string]string{
	"decided":            "DECISION",
	"chose":              "DECISION",
	"picked":             "DECISION",
	"selected":           "DECISION",
	"went with":          "DECISION",
	"settled on":         "DECISION",
	"switched":           "DECISION",
	"migrated":           "DECISION",
	"replaced":           "DECISION",
	"instead of":         "DECISION",
	"because":            "DECISION",
	"blocked":            "BLOCKER",
	"stuck":              "BLOCKER",
	"can't":              "BLOCKER",
	"cannot":             "BLOCKER",
	"impossible":         "BLOCKER",
	"preventing":         "BLOCKER",
	"shipped":            "MILESTONE",
	"launched":           "ORIGIN",
	"completed":          "MILESTONE",
	"finished":           "MILESTONE",
	"achieved":           "MILESTONE",
	"deployed":           "MILESTONE",
	"released":           "MILESTONE",
	"wondering":          "QUESTION",
	"unsure":             "QUESTION",
	"should we":          "QUESTION",
	"what if":            "QUESTION",
	"how do we":          "QUESTION",
	"risky":              "RISK",
	"dangerous":          "RISK",
	"concerned about":    "RISK",
	"might fail":         "RISK",
	"founded":            "ORIGIN",
	"created":            "ORIGIN",
	"started":            "ORIGIN",
	"born":               "ORIGIN",
	"first time":         "ORIGIN",
	"core":               "CORE",
	"fundamental":        "CORE",
	"essential":          "CORE",
	"principle":          "CORE",
	"belief":             "CORE",
	"turning point":      "PIVOT",
	"changed everything": "PIVOT",
	"realized":           "PIVOT",
	"breakthrough":       "PIVOT",
	"epiphany":           "PIVOT",
	"api":                "TECHNICAL",
	"database":           "TECHNICAL",
	"architecture":       "TECHNICAL",
	"deploy":             "TECHNICAL",
	"infrastructure":     "TECHNICAL",
	"algorithm":          "TECHNICAL",
	"framework":          "TECHNICAL",
	"server":             "TECHNICAL",
	"config":             "TECHNICAL",
}

// stopWords is the set of common English stop words excluded from topic extraction.
var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "is": true, "are": true,
	"was": true, "were": true, "be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true,
	"did": true, "will": true, "would": true, "could": true, "should": true,
	"may": true, "might": true, "shall": true, "can": true,
	"to": true, "of": true, "in": true, "for": true, "on": true,
	"with": true, "at": true, "by": true, "from": true, "as": true,
	"into": true, "about": true, "between": true, "through": true,
	"during": true, "before": true, "after": true, "above": true,
	"below": true, "up": true, "down": true, "out": true, "off": true,
	"over": true, "under": true, "again": true, "further": true,
	"then": true, "once": true, "here": true, "there": true,
	"when": true, "where": true, "why": true, "how": true,
	"all": true, "each": true, "every": true, "both": true,
	"few": true, "more": true, "most": true, "other": true,
	"some": true, "such": true, "no": true, "nor": true, "not": true,
	"only": true, "own": true, "same": true, "so": true, "than": true,
	"too": true, "very": true, "just": true, "don": true, "now": true,
	"and": true, "but": true, "or": true, "if": true, "while": true,
	"that": true, "this": true, "these": true, "those": true,
	"it": true, "its": true, "i": true, "we": true, "you": true,
	"he": true, "she": true, "they": true, "me": true, "him": true,
	"her": true, "us": true, "them": true, "my": true, "your": true,
	"his": true, "our": true, "their": true, "what": true, "which": true,
	"who": true, "whom": true, "also": true, "much": true, "many": true,
	"like": true, "because": true, "since": true, "get": true, "got": true,
	"use": true, "used": true, "using": true, "make": true, "made": true,
	"thing": true, "things": true, "way": true, "well": true,
	"really": true, "want": true, "need": true,
}

var wordRe = regexp.MustCompile(`[a-zA-Z][a-zA-Z_-]{2,}`)
var sentSplitRe = regexp.MustCompile(`[.!?\n]+`)

// decisionWords used for scoring key sentences.
var decisionWords = []string{
	"decided", "because", "instead", "prefer", "switched", "chose",
	"realized", "important", "key", "critical", "discovered", "learned",
	"conclusion", "solution", "reason", "why", "breakthrough", "insight",
}

// Compress takes plain text and returns an AAAK-compressed result.
func (d *Dialect) Compress(text string) CompressResult {
	if strings.TrimSpace(text) == "" {
		return CompressResult{}
	}

	// 1. Replace entity names with codes in the working text
	working := text
	for name, code := range d.EntityCodes {
		if name == strings.ToLower(name) {
			// Skip lowercase duplicates; we replace case-insensitively below
			continue
		}
		working = strings.ReplaceAll(working, name, code)
	}

	// 2. Detect entities present in text
	entities := d.detectEntities(text)

	// 3. Extract topics
	topics := d.extractTopics(text, 5)

	// 4. Extract key sentences (from working text with codes applied)
	keySentences := d.extractKeySentences(working, 3)

	// 5. Detect emotions
	emotions := d.detectEmotions(text)

	// 6. Detect flags
	flags := d.detectFlags(text)

	// 7. Format AAAK block
	var lines []string

	if len(topics) > 0 {
		lines = append(lines, "[T: "+strings.Join(topics, ", ")+"]")
	}
	if len(emotions) > 0 {
		lines = append(lines, "[E: "+strings.Join(emotions, ", ")+"]")
	}
	if len(flags) > 0 {
		lines = append(lines, "[F: "+strings.Join(flags, ", ")+"]")
	}
	if len(entities) > 0 {
		lines = append(lines, "[ENT: "+strings.Join(entities, ", ")+"]")
	}
	for _, s := range keySentences {
		lines = append(lines, s)
	}

	output := strings.Join(lines, "\n")
	inputLen := len(text)
	outputLen := len(output)
	ratio := 0.0
	if outputLen > 0 {
		ratio = float64(inputLen) / float64(outputLen)
	}

	return CompressResult{
		Text:      output,
		Topics:    topics,
		Emotions:  emotions,
		Flags:     flags,
		InputLen:  inputLen,
		OutputLen: outputLen,
		Ratio:     ratio,
	}
}

// detectEntities finds known entity codes in text, or detects capitalized names.
func (d *Dialect) detectEntities(text string) []string {
	var found []string
	seen := map[string]bool{}
	textLower := strings.ToLower(text)

	// Check known entities
	for name, code := range d.EntityCodes {
		if name == strings.ToLower(name) {
			continue // skip lowercase duplicates
		}
		if strings.Contains(textLower, strings.ToLower(name)) {
			if !seen[code] {
				found = append(found, code)
				seen[code] = true
			}
		}
	}
	if len(found) > 0 {
		sort.Strings(found)
		return found
	}

	// Fallback: find capitalized words that look like names
	words := strings.Fields(text)
	for idx, w := range words {
		clean := stripNonAlpha(w)
		if len(clean) < 2 || idx == 0 {
			continue
		}
		if clean[0] >= 'A' && clean[0] <= 'Z' && allLower(clean[1:]) && !stopWords[strings.ToLower(clean)] {
			code := strings.ToUpper(clean[:3])
			if !seen[code] {
				found = append(found, code)
				seen[code] = true
			}
			if len(found) >= 3 {
				break
			}
		}
	}
	return found
}

// extractTopics extracts key topic words by frequency.
func (d *Dialect) extractTopics(text string, maxTopics int) []string {
	words := wordRe.FindAllString(text, -1)
	freq := map[string]int{}

	for _, w := range words {
		wl := strings.ToLower(w)
		if stopWords[wl] || len(wl) < 3 {
			continue
		}
		freq[wl]++
	}

	// Boost proper nouns and technical terms
	for _, w := range words {
		wl := strings.ToLower(w)
		if stopWords[wl] {
			continue
		}
		if _, ok := freq[wl]; !ok {
			continue
		}
		if w[0] >= 'A' && w[0] <= 'Z' {
			freq[wl] += 2
		}
		if strings.Contains(w, "_") || strings.Contains(w, "-") || hasMidUpperCase(w) {
			freq[wl] += 2
		}
	}

	type wordCount struct {
		word  string
		count int
	}
	var ranked []wordCount
	for w, c := range freq {
		ranked = append(ranked, wordCount{w, c})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].count != ranked[j].count {
			return ranked[i].count > ranked[j].count
		}
		return ranked[i].word < ranked[j].word
	})

	var topics []string
	for i := 0; i < len(ranked) && i < maxTopics; i++ {
		topics = append(topics, ranked[i].word)
	}
	return topics
}

// extractKeySentences selects the top-N most important sentences.
func (d *Dialect) extractKeySentences(text string, n int) []string {
	parts := sentSplitRe.Split(text, -1)
	var sentences []string
	for _, s := range parts {
		s = strings.TrimSpace(s)
		if len(s) > 10 {
			sentences = append(sentences, s)
		}
	}
	if len(sentences) == 0 {
		return nil
	}

	type scored struct {
		score int
		text  string
	}
	var items []scored
	for _, s := range sentences {
		score := 0
		sl := strings.ToLower(s)
		for _, w := range decisionWords {
			if strings.Contains(sl, w) {
				score += 2
			}
		}
		// Prefer medium-length sentences
		if len(s) < 80 {
			score++
		}
		if len(s) < 40 {
			score++
		}
		if len(s) > 150 {
			score -= 2
		}
		items = append(items, scored{score, s})
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].score > items[j].score
	})

	var result []string
	for i := 0; i < len(items) && i < n; i++ {
		s := items[i].text
		if len(s) > 55 {
			s = s[:52] + "..."
		}
		result = append(result, s)
	}
	return result
}

// detectEmotions detects emotion codes from keyword signals in text.
func (d *Dialect) detectEmotions(text string) []string {
	tl := strings.ToLower(text)
	var detected []string
	seen := map[string]bool{}
	for keyword, code := range emotionSignals {
		if strings.Contains(tl, keyword) && !seen[code] {
			detected = append(detected, code)
			seen[code] = true
		}
		if len(detected) >= 3 {
			break
		}
	}
	sort.Strings(detected)
	return detected
}

// detectFlags detects importance flags from keyword signals in text.
func (d *Dialect) detectFlags(text string) []string {
	tl := strings.ToLower(text)
	var detected []string
	seen := map[string]bool{}
	for keyword, flag := range flagSignals {
		if strings.Contains(tl, keyword) && !seen[flag] {
			detected = append(detected, flag)
			seen[flag] = true
		}
		if len(detected) >= 5 {
			break
		}
	}
	sort.Strings(detected)
	return detected
}

// helpers

func stripNonAlpha(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func allLower(s string) bool {
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

func hasMidUpperCase(s string) bool {
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return false
}
