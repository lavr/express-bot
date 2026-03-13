package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Host   string      `yaml:"host"`
	BotID  string      `yaml:"bot_id"`
	Secret string      `yaml:"secret"`
	ChatID string      `yaml:"chat_id"`
	Cache  CacheConfig `yaml:"cache"`
}

type CacheConfig struct {
	Type      string `yaml:"type"`       // "none"|"file"|"vault"
	FilePath  string `yaml:"file_path"`
	VaultURL  string `yaml:"vault_url"`
	VaultPath string `yaml:"vault_path"`
	TTL       int    `yaml:"ttl"` // seconds, default 3600
}

// Flags holds CLI flag values for layering on top of file/env config.
type Flags struct {
	ConfigPath string
	Host       string
	BotID      string
	Secret     string
	ChatID     string
	NoCache    bool
}

// Load reads configuration with layering: YAML file → env vars → CLI flags.
func Load(flags Flags) (*Config, error) {
	cfg := &Config{
		Cache: CacheConfig{
			Type: "none",
			TTL:  3600,
		},
	}

	// Layer 1: YAML file
	configPath := flags.ConfigPath
	if configPath == "" {
		configPath = defaultConfigPath()
	}
	if data, err := os.ReadFile(configPath); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config %s: %w", configPath, err)
		}
	} else if flags.ConfigPath != "" {
		// User explicitly specified a config file — always error
		return nil, fmt.Errorf("reading config %s: %w", configPath, err)
	} else if !errors.Is(err, os.ErrNotExist) {
		// Default config exists but is unreadable — report the real cause
		return nil, fmt.Errorf("reading config %s: %w", configPath, err)
	}

	// Layer 2: environment variables
	applyEnv(cfg)

	// Layer 3: CLI flags (highest priority)
	applyFlags(cfg, flags)

	// Validate required fields
	if cfg.Host == "" {
		return nil, fmt.Errorf("host is required (--host, EXPRESS_HOST, or config file)")
	}
	if cfg.BotID == "" {
		return nil, fmt.Errorf("bot_id is required (--bot-id, EXPRESS_BOT_ID, or config file)")
	}
	if cfg.Secret == "" {
		return nil, fmt.Errorf("secret is required (--secret, EXPRESS_SECRET, or config file)")
	}

	return cfg, nil
}

// RequireChatID returns an error if ChatID is empty.
func (c *Config) RequireChatID() error {
	if c.ChatID == "" {
		return fmt.Errorf("chat_id is required (--chat-id, EXPRESS_CHAT_ID, or config file)")
	}
	return nil
}

func defaultConfigPath() string {
	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "express-send", "config.yaml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "express-send", "config.yaml")
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("EXPRESS_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("EXPRESS_BOT_ID"); v != "" {
		cfg.BotID = v
	}
	if v := os.Getenv("EXPRESS_SECRET"); v != "" {
		cfg.Secret = v
	}
	if v := os.Getenv("EXPRESS_CHAT_ID"); v != "" {
		cfg.ChatID = v
	}
	if v := os.Getenv("EXPRESS_CACHE_TYPE"); v != "" {
		cfg.Cache.Type = v
	}
	if v := os.Getenv("EXPRESS_CACHE_FILE_PATH"); v != "" {
		cfg.Cache.FilePath = v
	}
	if v := os.Getenv("EXPRESS_CACHE_VAULT_URL"); v != "" {
		cfg.Cache.VaultURL = v
	}
	if v := os.Getenv("EXPRESS_CACHE_VAULT_PATH"); v != "" {
		cfg.Cache.VaultPath = v
	}
	if v := os.Getenv("EXPRESS_CACHE_TTL"); v != "" {
		if ttl, err := strconv.Atoi(v); err == nil {
			cfg.Cache.TTL = ttl
		}
	}
}

func applyFlags(cfg *Config, flags Flags) {
	if flags.Host != "" {
		cfg.Host = flags.Host
	}
	if flags.BotID != "" {
		cfg.BotID = flags.BotID
	}
	if flags.Secret != "" {
		cfg.Secret = flags.Secret
	}
	if flags.ChatID != "" {
		cfg.ChatID = flags.ChatID
	}
	if flags.NoCache {
		cfg.Cache.Type = "none"
	}
}
