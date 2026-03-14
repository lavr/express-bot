//go:build vault

package secret_test

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/lavr/express-botx/internal/secret"
	"github.com/lavr/express-botx/internal/testutil"
)

func TestVaultResolve_ReadSecret(t *testing.T) {
	addr, tok := testutil.StartVault(t)
	path := fmt.Sprintf("secret/data/test-%d", rand.Int())

	testutil.WriteSecret(t, addr, tok, path, map[string]string{
		"bot_secret": "my-super-secret",
		"api_key":    "key-123",
	})

	t.Setenv("VAULT_ADDR", addr)
	t.Setenv("VAULT_TOKEN", tok)

	// Read existing key
	val, err := secret.Resolve(fmt.Sprintf("vault:%s#bot_secret", path))
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if val != "my-super-secret" {
		t.Errorf("Resolve() = %q, want %q", val, "my-super-secret")
	}

	// Read another key from same path
	val, err = secret.Resolve(fmt.Sprintf("vault:%s#api_key", path))
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if val != "key-123" {
		t.Errorf("Resolve() = %q, want %q", val, "key-123")
	}
}

func TestVaultResolve_MissingKey(t *testing.T) {
	addr, tok := testutil.StartVault(t)
	path := fmt.Sprintf("secret/data/test-%d", rand.Int())

	testutil.WriteSecret(t, addr, tok, path, map[string]string{
		"existing": "value",
	})

	t.Setenv("VAULT_ADDR", addr)
	t.Setenv("VAULT_TOKEN", tok)

	_, err := secret.Resolve(fmt.Sprintf("vault:%s#nonexistent", path))
	if err == nil {
		t.Error("expected error for missing key, got nil")
	}
}

func TestVaultResolve_MissingPath(t *testing.T) {
	addr, tok := testutil.StartVault(t)

	t.Setenv("VAULT_ADDR", addr)
	t.Setenv("VAULT_TOKEN", tok)

	_, err := secret.Resolve("vault:secret/data/does-not-exist#key")
	if err == nil {
		t.Error("expected error for missing path, got nil")
	}
}
