package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad_FromYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
host: express.example.com
bot_id: bot-123
secret: my-secret
chat_id: chat-456
cache:
  type: file
  file_path: /tmp/token.cache
  ttl: 1800
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(Flags{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Host != "express.example.com" {
		t.Errorf("Host = %q, want %q", cfg.Host, "express.example.com")
	}
	if cfg.BotID != "bot-123" {
		t.Errorf("BotID = %q, want %q", cfg.BotID, "bot-123")
	}
	if cfg.Secret != "my-secret" {
		t.Errorf("Secret = %q, want %q", cfg.Secret, "my-secret")
	}
	if cfg.ChatID != "chat-456" {
		t.Errorf("ChatID = %q, want %q", cfg.ChatID, "chat-456")
	}
	if cfg.Cache.Type != "file" {
		t.Errorf("Cache.Type = %q, want %q", cfg.Cache.Type, "file")
	}
	if cfg.Cache.TTL != 1800 {
		t.Errorf("Cache.TTL = %d, want %d", cfg.Cache.TTL, 1800)
	}
}

func TestLoad_EnvOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
host: from-yaml.com
bot_id: yaml-bot
secret: yaml-secret
chat_id: yaml-chat
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("EXPRESS_HOST", "from-env.com")
	t.Setenv("EXPRESS_BOT_ID", "env-bot")

	cfg, err := Load(Flags{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Host != "from-env.com" {
		t.Errorf("Host = %q, want %q (env should override yaml)", cfg.Host, "from-env.com")
	}
	if cfg.BotID != "env-bot" {
		t.Errorf("BotID = %q, want %q (env should override yaml)", cfg.BotID, "env-bot")
	}
	// Non-overridden values should remain from YAML
	if cfg.Secret != "yaml-secret" {
		t.Errorf("Secret = %q, want %q", cfg.Secret, "yaml-secret")
	}
}

func TestLoad_FlagsOverrideAll(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
host: from-yaml.com
bot_id: yaml-bot
secret: yaml-secret
chat_id: yaml-chat
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("EXPRESS_HOST", "from-env.com")

	cfg, err := Load(Flags{
		ConfigPath: cfgPath,
		Host:       "from-flag.com",
	})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Host != "from-flag.com" {
		t.Errorf("Host = %q, want %q (flag should override all)", cfg.Host, "from-flag.com")
	}
}

func TestLoad_MissingRequired(t *testing.T) {
	tests := []struct {
		name  string
		flags Flags
		want  string
	}{
		{"no host", Flags{BotID: "b", Secret: "s"}, "host is required"},
		{"no bot_id", Flags{Host: "h", Secret: "s"}, "bot_id is required"},
		{"no secret", Flags{Host: "h", BotID: "b"}, "secret is required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(tt.flags)
			if err == nil {
				t.Fatal("expected error")
			}
			if got := err.Error(); !strings.Contains(got, tt.want) {
				t.Errorf("error = %q, should contain %q", got, tt.want)
			}
		})
	}
}

func TestRequireChatID(t *testing.T) {
	cfg := &Config{ChatID: ""}
	if err := cfg.RequireChatID(); err == nil {
		t.Fatal("expected error for empty ChatID")
	}

	cfg.ChatID = "some-chat"
	if err := cfg.RequireChatID(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_NoCacheFlag(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `
host: h
bot_id: b
secret: s
chat_id: c
cache:
  type: file
`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(Flags{ConfigPath: cfgPath, NoCache: true})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Cache.Type != "none" {
		t.Errorf("Cache.Type = %q, want %q (--no-cache should disable)", cfg.Cache.Type, "none")
	}
}

func TestLoad_MissingConfigFile_NotExplicit(t *testing.T) {
	// When no config path specified explicitly and default doesn't exist, should not error
	// (as long as required fields come from flags/env)
	cfg, err := Load(Flags{
		Host:   "h",
		BotID:  "b",
		Secret: "s",
		ChatID: "c",
	})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Host != "h" {
		t.Errorf("Host = %q, want %q", cfg.Host, "h")
	}
}

func TestLoad_MissingConfigFile_Explicit(t *testing.T) {
	_, err := Load(Flags{ConfigPath: "/nonexistent/config.yaml"})
	if err == nil {
		t.Fatal("expected error for explicitly specified missing config")
	}
}

func TestLoad_UnreadableConfigFile_Explicit(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("host: h"), 0644); err != nil {
		t.Fatal(err)
	}
	// Make unreadable
	if err := os.Chmod(cfgPath, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(cfgPath, 0644) })

	_, err := Load(Flags{ConfigPath: cfgPath})
	if err == nil {
		t.Fatal("expected error for unreadable config file")
	}
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("error = %q, want permission denied", err.Error())
	}
}

func TestLoad_DefaultCacheTTL(t *testing.T) {
	cfg, err := Load(Flags{
		Host:   "h",
		BotID:  "b",
		Secret: "s",
		ChatID: "c",
	})
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Cache.TTL != 3600 {
		t.Errorf("Cache.TTL = %d, want 3600 (default)", cfg.Cache.TTL)
	}
}
