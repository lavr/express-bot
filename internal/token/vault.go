package token

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/lavr/express-botx/internal/httputil"
)

const vaultTimeout = 5 * time.Second

// VaultCache stores tokens in HashiCorp Vault KV v2.
type VaultCache struct {
	URL   string // Vault address, e.g. "https://vault.example.com"
	Path  string // KV path, e.g. "secret/data/express-send/tokens"
	Token string // Vault token (from VAULT_TOKEN env)
}

var vaultHTTPClient = httputil.NewClient(vaultTimeout)

func (c *VaultCache) Get(ctx context.Context, key string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, vaultTimeout)
	defer cancel()

	url := fmt.Sprintf("%s/v1/%s", c.URL, c.Path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("X-Vault-Token", c.Token)

	resp, err := vaultHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", nil // no cached data yet
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("vault GET failed: HTTP %d", resp.StatusCode)
	}

	var vaultResp struct {
		Data struct {
			Data map[string]interface{} `json:"data"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&vaultResp); err != nil {
		return "", err
	}

	entry, ok := vaultResp.Data.Data[key]
	if !ok {
		return "", nil
	}

	// Check expiry
	entryMap, ok := entry.(map[string]interface{})
	if !ok {
		return "", nil
	}
	expiresStr, _ := entryMap["expires"].(string)
	if expiresStr != "" {
		expires, err := time.Parse(time.RFC3339, expiresStr)
		if err == nil && time.Now().After(expires) {
			return "", nil // expired
		}
	}

	token, _ := entryMap["token"].(string)
	return token, nil
}

func (c *VaultCache) Set(ctx context.Context, key string, token string, ttl time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, vaultTimeout)
	defer cancel()

	// First read existing data
	existing := make(map[string]interface{})
	url := fmt.Sprintf("%s/v1/%s", c.URL, c.Path)

	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err == nil {
		getReq.Header.Set("X-Vault-Token", c.Token)
		if resp, err := vaultHTTPClient.Do(getReq); err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var vaultResp struct {
					Data struct {
						Data map[string]interface{} `json:"data"`
					} `json:"data"`
				}
				json.NewDecoder(resp.Body).Decode(&vaultResp)
				if vaultResp.Data.Data != nil {
					existing = vaultResp.Data.Data
				}
			}
		}
	}

	existing[key] = map[string]interface{}{
		"token":   token,
		"expires": time.Now().Add(ttl).Format(time.RFC3339),
	}

	payload, err := json.Marshal(map[string]interface{}{
		"data": existing,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("X-Vault-Token", c.Token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := vaultHTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("vault POST failed: HTTP %d", resp.StatusCode)
	}

	return nil
}
