package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	PalacePath     string            `json:"palace_path"`
	CollectionName string            `json:"collection_name"`
	ConfigDir      string            `json:"-"`
	PeopleMap      map[string]string `json:"people_map"`
	TopicWings     map[string]string `json:"topic_wings"`
}

func Load(configDir string) Config {
	cfg := Config{
		PalacePath:     filepath.Join(configDir, "palace"),
		CollectionName: "mempalace_drawers",
		ConfigDir:      configDir,
		PeopleMap:      make(map[string]string),
		TopicWings:     make(map[string]string),
	}
	data, err := os.ReadFile(filepath.Join(configDir, "config.json"))
	if err == nil {
		json.Unmarshal(data, &cfg)
	}
	if v := os.Getenv("MEMPALACE_PALACE_PATH"); v != "" {
		cfg.PalacePath = v
	} else if v := os.Getenv("MEMPAL_PALACE_PATH"); v != "" {
		cfg.PalacePath = v
	}
	cfg.ConfigDir = configDir
	return cfg
}

func (c Config) Init() error {
	os.MkdirAll(c.ConfigDir, 0755)
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(c.ConfigDir, "config.json"), data, 0644)
}
