//go:build vault

package testutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"testing"
	"time"
)

const rootToken = "test-root-token"

// StartVault starts an OpenBao/Vault server in dev mode and returns the
// address and root token. The server is stopped automatically via t.Cleanup.
// If neither "bao" nor "vault" binary is found, the test is skipped.
func StartVault(t *testing.T) (addr, token string) {
	t.Helper()

	binary := findBinary()
	if binary == "" {
		t.Skip("neither bao nor vault binary found, skipping integration test")
	}

	port := freePort(t)
	listenAddr := fmt.Sprintf("127.0.0.1:%d", port)
	addr = fmt.Sprintf("http://%s", listenAddr)

	cmd := exec.Command(binary, "server", "-dev",
		"-dev-listen-address="+listenAddr,
		"-dev-root-token-id="+rootToken,
	)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", binary, err)
	}

	t.Cleanup(func() {
		cmd.Process.Kill()
		cmd.Wait()
	})

	waitReady(t, addr, 10*time.Second)

	return addr, rootToken
}

// WriteSecret writes a KV v2 secret via HTTP API.
func WriteSecret(t *testing.T, addr, token, path string, data map[string]string) {
	t.Helper()

	payload, _ := json.Marshal(map[string]any{
		"data": data,
	})

	url := fmt.Sprintf("%s/v1/%s", addr, path)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	req.Header.Set("X-Vault-Token", token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("write secret: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Fatalf("write secret: HTTP %d", resp.StatusCode)
	}
}

func findBinary() string {
	for _, name := range []string{"bao", "vault"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port
}

func waitReady(t *testing.T, addr string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	url := addr + "/v1/sys/health"
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == 200 {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("vault/bao not ready after %s", timeout)
}
