package search

import "testing"

func TestClassifyPreference(t *testing.T) {
	tests := []string{
		"Can you suggest some accessories for my camera?",
		"What should I serve for dinner this weekend?",
		"Can you recommend some recent publications?",
		"Any tips for improving battery life?",
	}
	for _, q := range tests {
		if ClassifyQuery(q) != QueryPreference {
			t.Errorf("expected QueryPreference for %q", q)
		}
	}
}

func TestClassifyTemporal(t *testing.T) {
	tests := []string{
		"What did I do two weeks ago?",
		"What was the milestone I mentioned last month?",
		"How many months passed between graduation and my first job?",
		"When did I start learning piano?",
	}
	for _, q := range tests {
		if ClassifyQuery(q) != QueryTemporal {
			t.Errorf("expected QueryTemporal for %q", q)
		}
	}
}

func TestClassifyFact(t *testing.T) {
	tests := []string{
		"What degree did I graduate with?",
		"What breed is my dog?",
		"Who gave me a stand mixer?",
		"What is my ethnicity?",
	}
	for _, q := range tests {
		if ClassifyQuery(q) != QueryFact {
			t.Errorf("expected QueryFact for %q", q)
		}
	}
}
