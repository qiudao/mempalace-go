// Package normalize converts chat exports (Claude JSON, ChatGPT, Slack, plain text)
// to a unified transcript format.
package normalize

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Normalize reads a file and converts it to unified transcript format.
// Detection is based on file extension: .json, .jsonl, or plain text fallback.
func Normalize(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return "", nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return normalizeJSON(content)
	case ".jsonl":
		return normalizeJSONL(content)
	default:
		return content, nil
	}
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	User    string `json:"user"`
	Text    string `json:"text"`
}

func normalizeJSON(content string) (string, error) {
	// Try array format: Claude JSON or Slack
	var msgs []message
	if err := json.Unmarshal([]byte(content), &msgs); err == nil && len(msgs) > 0 {
		if msgs[0].Role != "" {
			return messagesToTranscript(msgs), nil
		}
		if msgs[0].User != "" || msgs[0].Text != "" {
			return slackToTranscript(msgs), nil
		}
	}

	// Try ChatGPT format: {"mapping": {...}}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &obj); err == nil {
		if mapping, ok := obj["mapping"]; ok {
			return parseChatGPT(mapping)
		}
	}

	// Fallback: return as plain text
	return content, nil
}

func normalizeJSONL(content string) (string, error) {
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var msg message
		if err := json.Unmarshal([]byte(line), &msg); err == nil && msg.Role != "" {
			lines = append(lines, formatMessage(msg.Role, msg.Content))
		}
	}
	return strings.Join(lines, "\n"), nil
}

func formatMessage(role, content string) string {
	if role == "user" {
		return fmt.Sprintf("> %s: %s", role, content)
	}
	return fmt.Sprintf("%s: %s", role, content)
}

func messagesToTranscript(msgs []message) string {
	var lines []string
	for _, m := range msgs {
		if m.Role != "" {
			lines = append(lines, formatMessage(m.Role, m.Content))
		}
	}
	return strings.Join(lines, "\n")
}

func slackToTranscript(msgs []message) string {
	var lines []string
	for _, m := range msgs {
		if m.Text == "" {
			continue
		}
		user := m.User
		if user == "" {
			user = "unknown"
		}
		lines = append(lines, fmt.Sprintf("%s: %s", user, m.Text))
	}
	return strings.Join(lines, "\n")
}

func parseChatGPT(mapping json.RawMessage) (string, error) {
	var nodes map[string]struct {
		Message *struct {
			Author struct {
				Role string `json:"role"`
			} `json:"author"`
			Content struct {
				Parts []any `json:"parts"`
			} `json:"content"`
		} `json:"message"`
		Children []string `json:"children"`
		Parent   string   `json:"parent"`
	}
	if err := json.Unmarshal(mapping, &nodes); err != nil {
		return string(mapping), nil
	}

	// Find root node (no parent)
	var rootID string
	for id, node := range nodes {
		if node.Parent == "" {
			rootID = id
			break
		}
	}
	if rootID == "" {
		return "", nil
	}

	var lines []string
	var walk func(id string)
	walk = func(id string) {
		node, ok := nodes[id]
		if !ok {
			return
		}
		if node.Message != nil {
			role := node.Message.Author.Role
			for _, part := range node.Message.Content.Parts {
				if s, ok := part.(string); ok && strings.TrimSpace(s) != "" {
					if role == "user" || role == "assistant" {
						lines = append(lines, formatMessage(role, s))
					}
				}
			}
		}
		for _, childID := range node.Children {
			walk(childID)
		}
	}
	walk(rootID)
	return strings.Join(lines, "\n"), nil
}
