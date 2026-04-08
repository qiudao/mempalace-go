package miner

import (
	"path/filepath"
	"strings"
)

// RoomConfig represents a user-configured room in the memory palace.
type RoomConfig struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

// folderRoomMap maps folder name keywords to room names.
// Ported from Python room_detector_local.py.
var folderRoomMap = map[string]string{
	"frontend": "frontend", "front-end": "frontend", "front_end": "frontend",
	"client": "frontend", "ui": "frontend", "views": "frontend",
	"components": "frontend", "pages": "frontend",
	"backend": "backend", "back-end": "backend", "back_end": "backend",
	"server": "backend", "api": "backend", "routes": "backend",
	"services": "backend", "controllers": "backend", "models": "backend",
	"database": "backend", "db": "backend",
	"docs": "documentation", "doc": "documentation", "documentation": "documentation",
	"wiki": "documentation", "readme": "documentation", "notes": "documentation",
	"design": "design", "designs": "design", "mockups": "design",
	"wireframes": "design", "assets": "design", "storyboard": "design",
	"costs": "costs", "cost": "costs", "budget": "costs",
	"finance": "costs", "financial": "costs", "pricing": "costs",
	"invoices": "costs", "accounting": "costs",
	"meetings": "meetings", "meeting": "meetings", "calls": "meetings",
	"meeting_notes": "meetings", "standup": "meetings", "minutes": "meetings",
	"team": "team", "staff": "team", "hr": "team",
	"hiring": "team", "employees": "team", "people": "team",
	"research": "research", "references": "research",
	"reading": "research", "papers": "research",
	"planning": "planning", "roadmap": "planning",
	"strategy": "planning", "specs": "planning", "requirements": "planning",
	"tests": "testing", "test": "testing", "testing": "testing", "qa": "testing",
	"scripts": "scripts", "tools": "scripts", "utils": "scripts",
	"config": "configuration", "configs": "configuration",
	"settings": "configuration", "infrastructure": "configuration",
	"infra": "configuration", "deploy": "configuration",
}

// DetectRoom determines which "room" a file belongs to based on its path
// segments and optional room configuration. If rooms is non-empty, configured
// room names are checked first. Falls back to the built-in folderRoomMap,
// then returns "general" if no match is found.
func DetectRoom(path string, rooms []RoomConfig) string {
	parts := strings.Split(filepath.ToSlash(path), "/")

	// Check against configured rooms first.
	if len(rooms) > 0 {
		for _, part := range parts {
			lower := strings.ToLower(part)
			for _, r := range rooms {
				if strings.ToLower(r.Name) == lower {
					return r.Name
				}
			}
		}
	}

	// Check folder-to-room keyword mapping.
	for _, part := range parts {
		lower := strings.ToLower(part)
		if room, ok := folderRoomMap[lower]; ok {
			return room
		}
	}

	return "general"
}
