package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mempalace/mempalace-go/internal/config"
	"github.com/mempalace/mempalace-go/internal/embed"
	"github.com/mempalace/mempalace-go/internal/layers"
	"github.com/mempalace/mempalace-go/internal/miner"
	"github.com/mempalace/mempalace-go/internal/search"
	"github.com/mempalace/mempalace-go/internal/store"
)

var (
	mineMode   string // "projects" or "convos"
	wing       string
	room       string
	limit      int
	embedModel string // ollama model name
)

func init() {
	// init command
	initCmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize palace config in ~/.mempalace",
		RunE:  runInit,
	}
	rootCmd.AddCommand(initCmd)

	// mine command
	mineCmd := &cobra.Command{
		Use:   "mine <directory>",
		Short: "Mine projects or conversations into the palace",
		Args:  cobra.ExactArgs(1),
		RunE:  runMine,
	}
	mineCmd.Flags().StringVar(&mineMode, "mode", "projects", "Mining mode: projects or convos")
	mineCmd.Flags().StringVar(&wing, "wing", "", "Wing name (for convos mode)")
	mineCmd.Flags().StringVar(&embedModel, "embed-model", "", "Ollama embedding model (default: nomic-embed-text)")
	rootCmd.AddCommand(mineCmd)

	// search command
	searchCmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search the palace",
		Args:  cobra.ExactArgs(1),
		RunE:  runSearch,
	}
	searchCmd.Flags().StringVar(&wing, "wing", "", "Filter by wing")
	searchCmd.Flags().StringVar(&room, "room", "", "Filter by room")
	searchCmd.Flags().IntVar(&limit, "limit", 10, "Max results")
	rootCmd.AddCommand(searchCmd)

	// wake-up command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "wake-up",
		Short: "Show identity + essential story (L0 + L1)",
		RunE:  runWakeUp,
	})

	// status command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show what's in the palace",
		RunE:  runStatus,
	})

	// compress command
	rootCmd.AddCommand(&cobra.Command{
		Use:   "compress",
		Short: "Compress drawers using AAAK dialect",
		RunE:  runCompress,
	})
}

func loadConfig() config.Config {
	home, _ := os.UserHomeDir()
	return config.Load(filepath.Join(home, ".mempalace"))
}

func openStore() (*store.Store, error) {
	cfg := loadConfig()
	return store.Open(filepath.Join(cfg.PalacePath, "mempalace.db"))
}

func runInit(cmd *cobra.Command, args []string) error {
	cfg := loadConfig()
	if err := cfg.Init(); err != nil {
		return fmt.Errorf("init config: %w", err)
	}
	// Ensure palace directory exists
	if err := os.MkdirAll(cfg.PalacePath, 0755); err != nil {
		return fmt.Errorf("create palace dir: %w", err)
	}
	fmt.Printf("Initialized palace at %s\n", cfg.PalacePath)
	fmt.Printf("Config at %s\n", cfg.ConfigDir)
	return nil
}

// tryEmbedder tries Ollama first, then ONNX, returns nil if neither available.
func tryEmbedder() (embed.EmbedderI, string) {
	// 1. Try Ollama (best: multilingual, any model)
	model := embedModel
	if model == "" {
		model = "nomic-embed-text"
	}
	ollamaEmb, err := embed.NewOllamaEmbedder("", model)
	if err == nil {
		return ollamaEmb, "ollama/" + model
	}

	// 2. Try ONNX (fallback: English-focused MiniLM)
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".cache/chroma/onnx_models/all-MiniLM-L6-v2/onnx")
	if _, err := os.Stat(filepath.Join(dir, "model.onnx")); err == nil {
		onnxEmb, err := embed.NewEmbedder(dir)
		if err == nil {
			return onnxEmb, "onnx/MiniLM-L6-v2"
		}
	}

	return nil, ""
}

func runMine(cmd *cobra.Command, args []string) error {
	dir := args[0]
	cfg := loadConfig()

	emb, name := tryEmbedder()
	if emb != nil {
		defer emb.Close()
		fmt.Printf("Embedder: %s — mining with vector embeddings\n", name)
	} else {
		fmt.Println("No embedder available — mining with text only (BM25 search)")
	}

	if mineMode == "convos" {
		wingName := wing
		if wingName == "" {
			wingName = filepath.Base(dir)
		}
		fmt.Printf("Mining conversations from %s into wing %q...\n", dir, wingName)
		return miner.MineConvos(dir, cfg.PalacePath, wingName, emb)
	}
	fmt.Printf("Mining project from %s...\n", dir)
	return miner.Mine(dir, cfg.PalacePath, emb)
}

func runSearch(cmd *cobra.Command, args []string) error {
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	// Try smart search (vector + BM25) if embedder is available
	emb, _ := tryEmbedder()
	if emb != nil {
		defer emb.Close()
		queryVec, err := emb.Embed(args[0])
		if err == nil {
			results, err := search.SmartSearch(s, args[0], queryVec, limit, store.Query{Wing: wing, Room: room})
			if err == nil && len(results) > 0 {
				for i, r := range results {
					fmt.Printf("\n--- Result %d [%s/%s] ---\n", i+1, r.Wing, r.Room)
					fmt.Printf("Source: %s | Filed: %s\n", r.Source, r.FiledAt)
					doc := r.Document
					if len(doc) > 500 {
						doc = doc[:500] + "..."
					}
					fmt.Println(doc)
				}
				fmt.Printf("\n%d result(s) found. (smart search)\n", len(results))
				return nil
			}
		}
	}

	// Fallback to BM25
	return search.Search(s, args[0], limit, wing, room)
}

func runWakeUp(cmd *cobra.Command, args []string) error {
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	cfg := loadConfig()
	stack := &layers.MemoryStack{
		Store:     s,
		ConfigDir: cfg.ConfigDir,
	}
	text, err := stack.WakeUp()
	if err != nil {
		return err
	}
	fmt.Println(text)
	return nil
}

func runStatus(cmd *cobra.Command, args []string) error {
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()

	cfg := loadConfig()
	stack := &layers.MemoryStack{
		Store:     s,
		ConfigDir: cfg.ConfigDir,
	}
	text, err := stack.Status()
	if err != nil {
		return err
	}
	fmt.Print(text)
	return nil
}

func runCompress(cmd *cobra.Command, args []string) error {
	fmt.Println("Compress: not yet implemented (requires LLM integration)")
	return nil
}
