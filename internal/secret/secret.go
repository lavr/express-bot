package secret

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/lavr/express-botx/internal/httputil"
)

// Resolve resolves a secret value from one of:
// - "env:VAR_NAME" → reads from environment variable
// - "vault:path#key" → reads from Vault KV v2 (requires VAULT_ADDR and VAULT_TOKEN)
// - anything else → returned as-is (literal value)
func Resolve(secret string) (string, error) {
	switch {
	case strings.HasPrefix(secret, "env:"):
		return resolveEnv(secret[4:])
	case strings.HasPrefix(secret, "vault:"):
		return resolveVault(secret[6:])
	default:
		return secret, nil
	}
}

func resolveEnv(varName string) (string, error) {
	val := os.Getenv(varName)
	if val == "" {
		return "", fmt.Errorf("environment variable %q is empty or not set", varName)
	}
	return val, nil
}

func resolveVault(spec string) (string, error) {
	// spec format: "path#key" or "secret/data/myapp#field"
	parts := strings.SplitN(spec, "#", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("vault secret spec must be path#key, got %q", spec)
	}
	path, key := parts[0], parts[1]

	vaultAddr := os.Getenv("VAULT_ADDR")
	if vaultAddr == "" {
		return "", fmt.Errorf("VAULT_ADDR not set")
	}
	vaultToken := os.Getenv("VAULT_TOKEN")
	if vaultToken == "" {
		return "", fmt.Errorf("VAULT_TOKEN not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	url := fmt.Sprintf("%s/v1/%s", strings.TrimRight(vaultAddr, "/"), path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("creating vault request: %w", err)
	}
	req.Header.Set("X-Vault-Token", vaultToken)

	client := httputil.NewClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("vault request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vault request failed: HTTP %d", resp.StatusCode)
	}

	var vaultResp struct {
		Data struct {
			Data map[string]interface{} `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&vaultResp); err != nil {
		return "", fmt.Errorf("decoding vault response: %w", err)
	}

	val, ok := vaultResp.Data.Data[key]
	if !ok {
		return "", fmt.Errorf("key %q not found in vault path %q", key, path)
	}

	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("vault value for key %q is not a string", key)
	}

	return str, nil
}
