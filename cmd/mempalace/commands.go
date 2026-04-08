package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mempalace/mempalace-go/internal/config"
	"github.com/mempalace/mempalace-go/internal/layers"
	"github.com/mempalace/mempalace-go/internal/miner"
	"github.com/mempalace/mempalace-go/internal/search"
	"github.com/mempalace/mempalace-go/internal/store"
)

var (
	mineMode string // "projects" or "convos"
	wing     string
	room     string
	limit    int
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

func runMine(cmd *cobra.Command, args []string) error {
	dir := args[0]
	cfg := loadConfig()

	if mineMode == "convos" {
		wingName := wing
		if wingName == "" {
			wingName = filepath.Base(dir)
		}
		fmt.Printf("Mining conversations from %s into wing %q...\n", dir, wingName)
		return miner.MineConvos(dir, cfg.PalacePath, wingName)
	}
	fmt.Printf("Mining project from %s...\n", dir)
	return miner.Mine(dir, cfg.PalacePath)
}

func runSearch(cmd *cobra.Command, args []string) error {
	s, err := openStore()
	if err != nil {
		return err
	}
	defer s.Close()
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
