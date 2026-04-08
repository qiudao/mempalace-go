package miner

import (
	"crypto/md5"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mempalace/mempalace-go/internal/normalize"
	"github.com/mempalace/mempalace-go/internal/store"
	"gopkg.in/yaml.v3"
)

// ProjectConfig represents the mempalace.yaml project configuration.
type ProjectConfig struct {
	Wing  string       `yaml:"wing"`
	Rooms []RoomConfig `yaml:"rooms"`
}

// skipDirs lists directory names that should be skipped during mining.
var skipDirs = map[string]bool{
	".git": true, "node_modules": true, "__pycache__": true,
	".venv": true, "venv": true, "env": true,
	"dist": true, "build": true, ".next": true, "coverage": true,
}

// binaryExts lists file extensions for binary files that should be skipped.
var binaryExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".ico": true, ".svg": true, ".woff": true, ".woff2": true,
	".ttf": true, ".eot": true, ".mp3": true, ".mp4": true,
	".zip": true, ".tar": true, ".gz": true, ".exe": true,
	".dll": true, ".so": true, ".dylib": true, ".pyc": true,
	".o": true, ".a": true, ".pdf": true,
}

// Mine walks a project directory, normalizes and chunks each text file,
// detects the room from its path, and stores the resulting drawers in
// a SQLite database at palacePath/mempalace.db.
// Embedder is the interface for generating vector embeddings.
type Embedder interface {
	Embed(text string) ([]float32, error)
	EmbedBatch(texts []string) ([][]float32, error)
	Close()
}

// Mine indexes project files into the palace. If embedder is non-nil,
// also generates vector embeddings for semantic search.
func Mine(projectDir, palacePath string, embedder ...Embedder) error {
	var emb Embedder
	if len(embedder) > 0 {
		emb = embedder[0]
	}
	return mineImpl(projectDir, palacePath, emb)
}

func mineImpl(projectDir, palacePath string, emb Embedder) error {
	// Read config
	cfgData, err := os.ReadFile(filepath.Join(projectDir, "mempalace.yaml"))
	if err != nil {
		return fmt.Errorf("missing mempalace.yaml: %w", err)
	}
	var cfg ProjectConfig
	if err := yaml.Unmarshal(cfgData, &cfg); err != nil {
		return fmt.Errorf("invalid mempalace.yaml: %w", err)
	}

	// Open store
	if err := os.MkdirAll(palacePath, 0755); err != nil {
		return fmt.Errorf("creating palace dir: %w", err)
	}
	s, err := store.Open(filepath.Join(palacePath, "mempalace.db"))
	if err != nil {
		return err
	}
	defer s.Close()

	now := time.Now().Format("2006-01-02")

	// Walk files
	return filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors
		}

		name := info.Name()

		if info.IsDir() {
			// Skip the palace output directory itself
			if path == palacePath {
				return filepath.SkipDir
			}
			if skipDirs[name] || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files
		if strings.HasPrefix(name, ".") {
			return nil
		}

		// Skip binary files
		ext := strings.ToLower(filepath.Ext(name))
		if binaryExts[ext] {
			return nil
		}

		// Skip the config file itself
		if name == "mempalace.yaml" {
			return nil
		}

		// Normalize content
		content, err := normalize.Normalize(path)
		if err != nil || content == "" {
			return nil
		}

		// Detect room from relative path
		relPath, _ := filepath.Rel(projectDir, path)
		room := DetectRoom(relPath, cfg.Rooms)

		// Chunk and store
		chunks := ChunkText(content, 800, 100)
		for _, chunk := range chunks {
			id := fmt.Sprintf("%x", md5.Sum([]byte(cfg.Wing+room+chunk)))
			drawer := store.Drawer{
				ID:       id,
				Document: chunk,
				Wing:     cfg.Wing,
				Room:     room,
				Source:   relPath,
				FiledAt:  now,
			}
			if emb != nil {
				vec, err := emb.Embed(chunk)
				if err == nil {
					s.AddWithEmbedding(drawer, vec)
				} else {
					s.Add(drawer) // fallback to text-only
				}
			} else {
				s.Add(drawer)
			}
		}
		return nil
	})
}
