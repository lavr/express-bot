package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testConfigYAML = `bots:
  test:
    host: express.example.com
    id: bot-123
    secret: env:BOT_SECRET
`

func TestConfigEdit_FileNotFound(t *testing.T) {
	deps, _, _ := testDeps()
	err := runConfigEdit([]string{"--config", "/nonexistent/path/config.yaml"}, deps)
	if err == nil {
		t.Fatal("expected error for missing config file")
	}
	if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "reading config") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigEdit_NoChanges(t *testing.T) {
	configPath := writeTestConfig(t, testConfigYAML)
	t.Setenv("EDITOR", "true")

	var stderr bytes.Buffer
	deps := Deps{
		Stdout:     &bytes.Buffer{},
		Stderr:     &stderr,
		Stdin:      strings.NewReader(""),
		IsTerminal: false,
	}

	err := runConfigEdit([]string{"--config", configPath}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "no changes") {
		t.Fatalf("expected 'no changes' message, got: %s", stderr.String())
	}
}

func TestConfigEdit_ValidEdit(t *testing.T) {
	configPath := writeTestConfig(t, testConfigYAML)

	newContent := `bots:
  updated:
    host: express.example.com
    id: bot-123
    secret: env:BOT_SECRET
`

	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "editor.sh")
	script := "#!/bin/sh\ncat > \"$1\" << 'ENDOFCONTENT'\n" + newContent + "ENDOFCONTENT\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", scriptPath)

	var stderr bytes.Buffer
	deps := Deps{
		Stdout:     &bytes.Buffer{},
		Stderr:     &stderr,
		Stdin:      strings.NewReader(""),
		IsTerminal: false,
	}

	err := runConfigEdit([]string{"--config", configPath}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "Config updated") {
		t.Fatalf("expected 'Config updated' message, got: %s", stderr.String())
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "updated") {
		t.Fatalf("config file was not updated, content: %s", string(data))
	}
}

func TestConfigEdit_InvalidYAML_Discard(t *testing.T) {
	configPath := writeTestConfig(t, testConfigYAML)

	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "editor.sh")
	script := "#!/bin/sh\necho 'invalid: yaml: content: [broken' > \"$1\"\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", scriptPath)

	var stderr bytes.Buffer
	deps := Deps{
		Stdout:     &bytes.Buffer{},
		Stderr:     &stderr,
		Stdin:      strings.NewReader("d\n"),
		IsTerminal: false,
	}

	err := runConfigEdit([]string{"--config", configPath}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "discarded") {
		t.Fatalf("expected 'discarded' message, got: %s", stderr.String())
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != testConfigYAML {
		t.Fatalf("config file was modified: %s", string(data))
	}
}

func TestConfigEdit_EditorFailure(t *testing.T) {
	configPath := writeTestConfig(t, testConfigYAML)
	t.Setenv("EDITOR", "false")

	var stderr bytes.Buffer
	deps := Deps{
		Stdout:     &bytes.Buffer{},
		Stderr:     &stderr,
		Stdin:      strings.NewReader(""),
		IsTerminal: false,
	}

	err := runConfigEdit([]string{"--config", configPath}, deps)
	if err == nil {
		t.Fatal("expected error when editor exits with non-zero status")
	}
	if !strings.Contains(err.Error(), "editor exited with error") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigEdit_InvalidYAML_Retry(t *testing.T) {
	configPath := writeTestConfig(t, testConfigYAML)

	newContent := `bots:
  retried:
    host: express.example.com
    id: bot-123
    secret: env:BOT_SECRET
`

	scriptDir := t.TempDir()
	scriptPath := filepath.Join(scriptDir, "editor.sh")
	stateFile := filepath.Join(scriptDir, "state")

	// First invocation writes invalid YAML, second writes valid YAML
	script := `#!/bin/sh
if [ ! -f "` + stateFile + `" ]; then
  echo 'invalid: yaml: [broken' > "$1"
  touch "` + stateFile + `"
else
  cat > "$1" << 'ENDOFCONTENT'
` + newContent + `ENDOFCONTENT
fi
`
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", scriptPath)

	var stderr bytes.Buffer
	deps := Deps{
		Stdout:     &bytes.Buffer{},
		Stderr:     &stderr,
		Stdin:      strings.NewReader("r\n"),
		IsTerminal: false,
	}

	err := runConfigEdit([]string{"--config", configPath}, deps)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(stderr.String(), "Validation error") {
		t.Fatalf("expected 'Validation error' message, got: %s", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Config updated") {
		t.Fatalf("expected 'Config updated' message, got: %s", stderr.String())
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "retried") {
		t.Fatalf("config file was not updated after retry, content: %s", string(data))
	}
}
