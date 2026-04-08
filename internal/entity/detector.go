// Package entity provides heuristic entity detection and a persistent entity registry.
package entity

import (
	"regexp"
	"strings"
)

// Entity represents a detected entity with classification.
type Entity struct {
	Name       string
	Type       string  // "person", "project", "uncertain"
	Confidence float64
}

// Person-signal verb patterns (name appears before the verb).
var personVerbPatterns = []string{
	`\b%s\s+said\b`,
	`\b%s\s+asked\b`,
	`\b%s\s+told\b`,
	`\b%s\s+replied\b`,
	`\b%s\s+laughed\b`,
	`\b%s\s+smiled\b`,
	`\b%s\s+cried\b`,
	`\b%s\s+felt\b`,
	`\b%s\s+thinks?\b`,
	`\b%s\s+wants?\b`,
	`\b%s\s+loves?\b`,
	`\b%s\s+hates?\b`,
	`\b%s\s+knows?\b`,
	`\b%s\s+decided\b`,
	`\b%s\s+pushed\b`,
	`\b%s\s+wrote\b`,
	`\bhey\s+%s\b`,
	`\bthanks?\s+%s\b`,
	`\bhi\s+%s\b`,
	`\bdear\s+%s\b`,
}

// Dialogue patterns (strong person signal).
var dialoguePatterns = []string{
	`(?m)^>\s*%s[:\s]`,
	`(?m)^%s:\s`,
	`(?m)^\[%s\]`,
	`"%s\s+said`,
}

// Project-signal verb patterns.
var projectVerbPatterns = []string{
	`\bbuilding\s+%s\b`,
	`\bbuilt\s+%s\b`,
	`\bship(?:ping|ped)?\s+%s\b`,
	`\blaunch(?:ing|ed)?\s+%s\b`,
	`\bdeploy(?:ing|ed)?\s+%s\b`,
	`\binstall(?:ing|ed)?\s+%s\b`,
	`\bthe\s+%s\s+architecture\b`,
	`\bthe\s+%s\s+pipeline\b`,
	`\bthe\s+%s\s+system\b`,
	`\bthe\s+%s\s+repo\b`,
	`\b%s\s+v\d+\b`,
	`\b%s\.(?:py|js|ts|yaml|yml|json|sh)\b`,
	`\b%s-core\b`,
	`\b%s-local\b`,
	`\bimport\s+%s\b`,
	`\bpip\s+install\s+%s\b`,
}

// Pronoun patterns for proximity check.
var pronounPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bshe\b`),
	regexp.MustCompile(`(?i)\bher\b`),
	regexp.MustCompile(`(?i)\bhers\b`),
	regexp.MustCompile(`(?i)\bhe\b`),
	regexp.MustCompile(`(?i)\bhim\b`),
	regexp.MustCompile(`(?i)\bhis\b`),
	regexp.MustCompile(`(?i)\bthey\b`),
	regexp.MustCompile(`(?i)\bthem\b`),
	regexp.MustCompile(`(?i)\btheir\b`),
}

// stopwords that should not be considered entity candidates.
var stopwords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
	"with": true, "by": true, "from": true, "as": true, "is": true, "was": true,
	"are": true, "were": true, "be": true, "been": true, "being": true,
	"have": true, "has": true, "had": true, "do": true, "does": true, "did": true,
	"will": true, "would": true, "could": true, "should": true, "may": true,
	"might": true, "must": true, "shall": true, "can": true,
	"this": true, "that": true, "these": true, "those": true, "it": true, "its": true,
	"they": true, "them": true, "their": true, "we": true, "our": true,
	"you": true, "your": true, "my": true, "me": true, "he": true, "she": true,
	"his": true, "her": true, "who": true, "what": true, "when": true, "where": true,
	"why": true, "how": true, "which": true, "if": true, "then": true, "so": true,
	"not": true, "no": true, "yes": true, "ok": true, "okay": true,
	"just": true, "very": true, "really": true, "also": true, "already": true,
	"still": true, "even": true, "only": true, "here": true, "there": true,
	"now": true, "too": true, "up": true, "out": true, "about": true, "like": true,
	"use": true, "get": true, "got": true, "make": true, "made": true,
	"take": true, "put": true, "come": true, "go": true, "see": true,
	"know": true, "think": true, "true": true, "false": true, "none": true,
	"null": true, "new": true, "old": true, "all": true, "any": true, "some": true,
	"step": true, "usage": true, "run": true, "check": true, "find": true,
	"add": true, "set": true, "list": true, "note": true, "example": true,
	"every": true, "each": true, "more": true, "less": true, "next": true,
	"last": true, "first": true, "second": true, "test": true, "stop": true,
	"start": true, "copy": true, "move": true, "source": true, "target": true,
	"output": true, "input": true, "data": true, "item": true, "key": true,
	"value": true, "world": true, "well": true, "want": true, "topic": true,
	"memory": true, "model": true, "models": true, "system": true, "version": true,
}

// candidateRe matches single capitalized words (2+ lowercase after capital).
var candidateRe = regexp.MustCompile(`\b([A-Z][a-z]{1,19})\b`)

// camelCaseRe matches CamelCase words like MemPalace, GitHub, etc.
var camelCaseRe = regexp.MustCompile(`\b([A-Z][a-z]+(?:[A-Z][a-z]+)+)\b`)

// extractCandidates returns candidate names with their frequency.
func extractCandidates(text string) map[string]int {
	counts := make(map[string]int)

	// Single capitalized words.
	for _, w := range candidateRe.FindAllString(text, -1) {
		if len(w) > 1 && !stopwords[strings.ToLower(w)] {
			counts[w]++
		}
	}

	// CamelCase words (e.g. MemPalace).
	for _, w := range camelCaseRe.FindAllString(text, -1) {
		counts[w]++
	}

	return counts
}

// countPattern returns how many times pattern (with %s replaced by name) matches text.
func countPattern(pattern, name, text string) int {
	escaped := regexp.QuoteMeta(name)
	full := strings.ReplaceAll(pattern, "%s", escaped)
	re, err := regexp.Compile("(?i)" + full)
	if err != nil {
		return 0
	}
	return len(re.FindAllString(text, -1))
}

type scores struct {
	personScore     int
	projectScore    int
	personSignals   []string
	projectSignals  []string
}

func scoreEntity(name, text string, lines []string) scores {
	var s scores

	// Dialogue markers (strong person signal).
	for _, pat := range dialoguePatterns {
		n := countPattern(pat, name, text)
		if n > 0 {
			s.personScore += n * 3
			s.personSignals = append(s.personSignals, "dialogue")
		}
	}

	// Person verbs.
	for _, pat := range personVerbPatterns {
		n := countPattern(pat, name, text)
		if n > 0 {
			s.personScore += n * 2
			s.personSignals = append(s.personSignals, "action")
		}
	}

	// Pronoun proximity.
	nameLower := strings.ToLower(name)
	var nameLineIndices []int
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), nameLower) {
			nameLineIndices = append(nameLineIndices, i)
		}
	}
	pronounHits := 0
	for _, idx := range nameLineIndices {
		start := idx - 2
		if start < 0 {
			start = 0
		}
		end := idx + 3
		if end > len(lines) {
			end = len(lines)
		}
		window := strings.ToLower(strings.Join(lines[start:end], " "))
		for _, rx := range pronounPatterns {
			if rx.MatchString(window) {
				pronounHits++
				break
			}
		}
	}
	if pronounHits > 0 {
		s.personScore += pronounHits * 2
		s.personSignals = append(s.personSignals, "pronoun")
	}

	// Direct address.
	escaped := regexp.QuoteMeta(name)
	directRe := regexp.MustCompile(`(?i)\bhey\s+` + escaped + `\b|\bthanks?\s+` + escaped + `\b|\bhi\s+` + escaped + `\b`)
	if n := len(directRe.FindAllString(text, -1)); n > 0 {
		s.personScore += n * 4
		s.personSignals = append(s.personSignals, "addressed")
	}

	// Project verbs.
	for _, pat := range projectVerbPatterns {
		n := countPattern(pat, name, text)
		if n > 0 {
			s.projectScore += n * 2
			s.projectSignals = append(s.projectSignals, "project verb")
		}
	}

	// Versioned / hyphenated.
	versionedRe := regexp.MustCompile(`(?i)\b` + escaped + `[-v]\w+`)
	if n := len(versionedRe.FindAllString(text, -1)); n > 0 {
		s.projectScore += n * 3
		s.projectSignals = append(s.projectSignals, "versioned")
	}

	// Code file reference.
	codeRefRe := regexp.MustCompile(`(?i)\b` + escaped + `\.(?:py|js|ts|yaml|yml|json|sh)\b`)
	if n := len(codeRefRe.FindAllString(text, -1)); n > 0 {
		s.projectScore += n * 3
		s.projectSignals = append(s.projectSignals, "code_ref")
	}

	return s
}

func classifyEntity(name string, frequency int, s scores) Entity {
	ps := s.personScore
	prs := s.projectScore
	total := ps + prs

	if total == 0 {
		conf := float64(frequency) / 50.0
		if conf > 0.4 {
			conf = 0.4
		}
		return Entity{Name: name, Type: "uncertain", Confidence: conf}
	}

	personRatio := float64(ps) / float64(total)

	// Count distinct signal categories.
	cats := make(map[string]bool)
	for _, sig := range s.personSignals {
		cats[sig] = true
	}
	hasTwoTypes := len(cats) >= 2

	switch {
	case personRatio >= 0.7 && hasTwoTypes && ps >= 2:
		conf := 0.5 + personRatio*0.5
		if conf > 0.99 {
			conf = 0.99
		}
		return Entity{Name: name, Type: "person", Confidence: conf}
	case personRatio >= 0.7 && ps >= 2:
		// Has person signals but only one signal category -- still classify as
		// person with slightly lower confidence for single-text detection.
		conf := 0.5 + personRatio*0.3
		if conf > 0.85 {
			conf = 0.85
		}
		return Entity{Name: name, Type: "person", Confidence: conf}
	case personRatio >= 0.7:
		return Entity{Name: name, Type: "uncertain", Confidence: 0.4}
	case personRatio <= 0.3:
		conf := 0.5 + (1-personRatio)*0.5
		if conf > 0.99 {
			conf = 0.99
		}
		return Entity{Name: name, Type: "project", Confidence: conf}
	default:
		return Entity{Name: name, Type: "uncertain", Confidence: 0.5}
	}
}

// DetectEntities scans a block of text and returns detected entities
// classified as person, project, or uncertain.
func DetectEntities(text string) []Entity {
	candidates := extractCandidates(text)
	if len(candidates) == 0 {
		return nil
	}

	lines := strings.Split(text, "\n")
	var results []Entity

	for name, freq := range candidates {
		s := scoreEntity(name, text, lines)
		e := classifyEntity(name, freq, s)
		results = append(results, e)
	}
	return results
}
