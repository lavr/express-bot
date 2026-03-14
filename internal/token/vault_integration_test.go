//go:build vault

package token_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/lavr/express-botx/internal/testutil"
	"github.com/lavr/express-botx/internal/token"
)

func vaultCache(t *testing.T) (*token.VaultCache, string) {
	t.Helper()
	addr, tok := testutil.StartVault(t)
	path := fmt.Sprintf("secret/data/test-%d", rand.Int())
	return &token.VaultCache{
		URL:   addr,
		Path:  path,
		Token: tok,
	}, addr
}

func TestVaultIntegration_SetGet(t *testing.T) {
	c, _ := vaultCache(t)
	ctx := context.Background()

	// Miss on empty
	val, err := c.Get(ctx, "bot1")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != "" {
		t.Errorf("Get() = %q, want empty on miss", val)
	}

	// Set and get
	if err := c.Set(ctx, "bot1", "real-vault-token", time.Hour); err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	val, err = c.Get(ctx, "bot1")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != "real-vault-token" {
		t.Errorf("Get() = %q, want %q", val, "real-vault-token")
	}
}

func TestVaultIntegration_Expiry(t *testing.T) {
	c, _ := vaultCache(t)
	ctx := context.Background()

	if err := c.Set(ctx, "bot1", "short-lived", 50*time.Millisecond); err != nil {
		t.Fatalf("Set() error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	val, err := c.Get(ctx, "bot1")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != "" {
		t.Errorf("Get() = %q, want empty (expired)", val)
	}
}

func TestVaultIntegration_MultipleKeys(t *testing.T) {
	c, _ := vaultCache(t)
	ctx := context.Background()

	c.Set(ctx, "bot1", "token-1", time.Hour)
	c.Set(ctx, "bot2", "token-2", time.Hour)
	c.Set(ctx, "bot3", "token-3", time.Hour)

	for i, expected := range []string{"token-1", "token-2", "token-3"} {
		key := fmt.Sprintf("bot%d", i+1)
		val, err := c.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get(%s) error: %v", key, err)
		}
		if val != expected {
			t.Errorf("Get(%s) = %q, want %q", key, val, expected)
		}
	}
}

func TestVaultIntegration_Overwrite(t *testing.T) {
	c, _ := vaultCache(t)
	ctx := context.Background()

	c.Set(ctx, "bot1", "old-token", time.Hour)
	c.Set(ctx, "bot1", "new-token", time.Hour)

	val, _ := c.Get(ctx, "bot1")
	if val != "new-token" {
		t.Errorf("Get() = %q, want %q after overwrite", val, "new-token")
	}
}

func TestVaultIntegration_WrongToken(t *testing.T) {
	c, _ := vaultCache(t)
	c.Token = "wrong-token"
	ctx := context.Background()

	_, err := c.Get(ctx, "bot1")
	if err == nil {
		t.Error("expected error with wrong token, got nil")
	}
}
