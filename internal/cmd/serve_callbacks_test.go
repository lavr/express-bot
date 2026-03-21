package cmd

import (
	"testing"

	"github.com/lavr/express-botx/internal/config"
)

func TestBuildBotSecretLookup_SingleBot(t *testing.T) {
	cfg := &config.Config{
		BotID:     "bot-123",
		BotSecret: "secret-abc",
	}
	lookup, err := buildBotSecretLookup(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Known bot
	sec, err := lookup("bot-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sec != "secret-abc" {
		t.Fatalf("expected secret-abc, got %s", sec)
	}

	// Unknown bot
	_, err = lookup("bot-unknown")
	if err == nil {
		t.Fatal("expected error for unknown bot_id")
	}
}

func TestBuildBotSecretLookup_NoSecret(t *testing.T) {
	cfg := &config.Config{
		BotID:    "bot-123",
		BotToken: "some-token",
	}
	lookup, err := buildBotSecretLookup(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = lookup("bot-123")
	if err == nil {
		t.Fatal("expected error for bot without secret")
	}
}

func TestBuildBotSecretLookup_MultiBot(t *testing.T) {
	cfg := &config.Config{
		Bots: map[string]config.BotConfig{
			"alpha": {ID: "bot-aaa", Secret: "secret-aaa"},
			"beta":  {ID: "bot-bbb", Secret: "secret-bbb"},
			"gamma": {ID: "bot-ccc", Token: "token-only"},
		},
	}
	cfg.SetMultiBot(true)
	lookup, err := buildBotSecretLookup(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Known bot with secret
	sec, err := lookup("bot-aaa")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sec != "secret-aaa" {
		t.Fatalf("expected secret-aaa, got %s", sec)
	}

	// Second known bot
	sec, err = lookup("bot-bbb")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sec != "secret-bbb" {
		t.Fatalf("expected secret-bbb, got %s", sec)
	}

	// Bot with no secret (token-only)
	_, err = lookup("bot-ccc")
	if err == nil {
		t.Fatal("expected error for bot without secret")
	}

	// Unknown bot
	_, err = lookup("bot-unknown")
	if err == nil {
		t.Fatal("expected error for unknown bot_id")
	}
}
