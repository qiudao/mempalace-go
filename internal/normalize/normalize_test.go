package normalize

import (
	"os"
	"strings"
	"testing"
)

func writeTmp(t *testing.T, name, content string) string {
	t.Helper()
	path := t.TempDir() + "/" + name
	os.WriteFile(path, []byte(content), 0644)
	return path
}

func TestPlainText(t *testing.T) {
	f := writeTmp(t, "test.txt", "Hello world\nSecond line\n")
	result, err := Normalize(f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Hello world") {
		t.Fatal("expected content preserved")
	}
}

func TestClaudeJSON(t *testing.T) {
	f := writeTmp(t, "test.json", `[{"role":"user","content":"Hi"},{"role":"assistant","content":"Hello"}]`)
	result, err := Normalize(f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "Hi") || !strings.Contains(result, "Hello") {
		t.Fatal("expected both messages in transcript")
	}
}

func TestEmpty(t *testing.T) {
	f := writeTmp(t, "test.txt", "")
	result, err := Normalize(f)
	if err != nil {
		t.Fatal(err)
	}
	if result != "" {
		t.Fatalf("expected empty, got %q", result)
	}
}

func TestClaudeJSONTranscriptFormat(t *testing.T) {
	f := writeTmp(t, "test.json", `[{"role":"user","content":"Hi"},{"role":"assistant","content":"Hello"}]`)
	result, err := Normalize(f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "> user: Hi") {
		t.Fatalf("expected '> user: Hi' prefix, got %q", result)
	}
	if !strings.Contains(result, "assistant: Hello") {
		t.Fatalf("expected 'assistant: Hello', got %q", result)
	}
}

func TestSlackJSON(t *testing.T) {
	f := writeTmp(t, "slack.json", `[{"user":"alice","text":"hey"},{"user":"bob","text":"hi"}]`)
	result, err := Normalize(f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "alice: hey") || !strings.Contains(result, "bob: hi") {
		t.Fatalf("expected slack messages, got %q", result)
	}
}

func TestChatGPTJSON(t *testing.T) {
	chatgpt := `{
		"mapping": {
			"root": {
				"message": null,
				"parent": "",
				"children": ["msg1"]
			},
			"msg1": {
				"message": {
					"author": {"role": "user"},
					"content": {"parts": ["What is Go?"]}
				},
				"parent": "root",
				"children": ["msg2"]
			},
			"msg2": {
				"message": {
					"author": {"role": "assistant"},
					"content": {"parts": ["Go is a programming language."]}
				},
				"parent": "msg1",
				"children": []
			}
		}
	}`
	f := writeTmp(t, "chatgpt.json", chatgpt)
	result, err := Normalize(f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "> user: What is Go?") {
		t.Fatalf("expected user message, got %q", result)
	}
	if !strings.Contains(result, "assistant: Go is a programming language.") {
		t.Fatalf("expected assistant message, got %q", result)
	}
}

func TestJSONL(t *testing.T) {
	content := `{"role":"user","content":"Hello"}
{"role":"assistant","content":"Hi there"}
{"role":"user","content":"Bye"}
`
	f := writeTmp(t, "test.jsonl", content)
	result, err := Normalize(f)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "> user: Hello") {
		t.Fatalf("expected user message, got %q", result)
	}
	if !strings.Contains(result, "assistant: Hi there") {
		t.Fatalf("expected assistant message, got %q", result)
	}
}

func TestFileNotFound(t *testing.T) {
	_, err := Normalize("/nonexistent/file.txt")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
