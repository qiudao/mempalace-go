package miner

import "testing"

func TestDetectRoomFromPath(t *testing.T) {
	tests := []struct{ path, want string }{
		{"backend/app.py", "backend"},
		{"server/routes/api.go", "backend"},
		{"frontend/components/Button.tsx", "frontend"},
		{"docs/readme.md", "documentation"},
		{"tests/unit_test.go", "testing"},
		{"scripts/deploy.sh", "scripts"},
		{"config/settings.yaml", "configuration"},
		{"random/stuff.txt", "general"},
	}
	for _, tt := range tests {
		got := DetectRoom(tt.path, nil)
		if got != tt.want {
			t.Errorf("DetectRoom(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestDetectRoomWithConfig(t *testing.T) {
	rooms := []RoomConfig{{Name: "api", Description: "API layer"}}
	got := DetectRoom("api/handlers.go", rooms)
	if got != "api" {
		t.Errorf("expected 'api', got %q", got)
	}
}
