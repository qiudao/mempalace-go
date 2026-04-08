package dialect

import (
	"strings"
	"testing"
)

func TestCompressBasic(t *testing.T) {
	d := New(map[string]string{"Alice": "A1", "Bob": "B1"})
	input := strings.Repeat("Alice and Bob discussed the API design. They decided to use GraphQL. ", 10)
	result := d.Compress(input)

	if result.OutputLen >= result.InputLen {
		t.Fatalf("no compression: output %d >= input %d", result.OutputLen, result.InputLen)
	}
	if !strings.Contains(result.Text, "A1") {
		t.Fatal("expected entity code A1 in output")
	}
}

func TestCompressEmotions(t *testing.T) {
	d := New(nil)
	result := d.Compress("I was really frustrated and anxious about the deadline. I felt hopeless and stuck.")
	hasEmotion := len(result.Emotions) > 0
	if !hasEmotion {
		t.Fatal("expected emotion detection")
	}
}

func TestCompressFlags(t *testing.T) {
	d := New(nil)
	result := d.Compress("We decided to use PostgreSQL. The migration was completed successfully. We shipped the new API.")
	if len(result.Flags) == 0 {
		t.Fatal("expected flag detection (decision, milestone)")
	}
}

func TestCompressTopics(t *testing.T) {
	d := New(nil)
	text := "The database migration requires careful planning. We need to migrate the database schema. The database performance is critical."
	result := d.Compress(text)
	if len(result.Topics) == 0 {
		t.Fatal("expected topic extraction")
	}
	// "database" should be a top topic
	found := false
	for _, topic := range result.Topics {
		if topic == "database" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'database' as topic, got %v", result.Topics)
	}
}

func TestCompressEmpty(t *testing.T) {
	d := New(nil)
	result := d.Compress("")
	if result.Text != "" {
		t.Fatal("expected empty result")
	}
}

func TestCompressRatio(t *testing.T) {
	d := New(nil)
	longText := strings.Repeat("The software engineering team discussed architecture patterns for the new microservices platform. ", 50)
	result := d.Compress(longText)
	if result.Ratio <= 1.0 {
		t.Fatalf("expected compression ratio > 1, got %.2f", result.Ratio)
	}
}

func TestEntityReplacement(t *testing.T) {
	d := New(map[string]string{"Alice": "A1", "Bob": "B1"})
	result := d.Compress("Alice told Bob that she loves the new design.")
	if !strings.Contains(result.Text, "A1") {
		t.Fatal("expected A1 in output")
	}
	if !strings.Contains(result.Text, "B1") {
		t.Fatal("expected B1 in output")
	}
}

func TestEmotionCodes(t *testing.T) {
	d := New(nil)
	result := d.Compress("I am so happy and excited about this project. I love working on it.")
	found := map[string]bool{}
	for _, e := range result.Emotions {
		found[e] = true
	}
	if !found["joy"] && !found["excite"] && !found["love"] {
		t.Fatalf("expected joy/excite/love emotions, got %v", result.Emotions)
	}
}

func TestFlagTypes(t *testing.T) {
	d := New(nil)

	// Decision
	r := d.Compress("We decided to switch from REST to GraphQL because of performance.")
	hasDecision := false
	for _, f := range r.Flags {
		if f == "DECISION" {
			hasDecision = true
		}
	}
	if !hasDecision {
		t.Fatalf("expected DECISION flag, got %v", r.Flags)
	}

	// Milestone
	r = d.Compress("We shipped the new API and it was deployed to production.")
	// shipped/deployed should trigger something
	if len(r.Flags) == 0 {
		t.Fatalf("expected flags for shipped/deployed, got %v", r.Flags)
	}
}

func TestTopicExtraction(t *testing.T) {
	d := New(nil)
	text := "Kubernetes cluster management requires monitoring. The Kubernetes pods need scaling. Kubernetes orchestration is complex."
	result := d.Compress(text)
	found := false
	for _, topic := range result.Topics {
		if topic == "kubernetes" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected 'kubernetes' as topic, got %v", result.Topics)
	}
}

func TestNilEntityCodes(t *testing.T) {
	d := New(nil)
	result := d.Compress("Some random text with no entities mentioned.")
	if result.Text == "" {
		t.Fatal("expected non-empty result for non-empty input")
	}
}
