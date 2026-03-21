package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	vlog "github.com/lavr/express-botx/internal/log"
)

// ExecHandler runs an external command to handle callback events.
// The callback payload JSON is passed via stdin.
type ExecHandler struct {
	command string
	timeout time.Duration
}

// NewExecHandler creates an ExecHandler that runs the given command
// with the specified timeout. A zero timeout means no deadline.
func NewExecHandler(command string, timeout time.Duration) *ExecHandler {
	return &ExecHandler{
		command: command,
		timeout: timeout,
	}
}

// Type returns "exec".
func (h *ExecHandler) Type() string {
	return "exec"
}

// callbackEnvMeta is used to extract metadata from callback payloads for env variables.
type callbackEnvMeta struct {
	SyncID string `json:"sync_id"`
	BotID  string `json:"bot_id"`
	From   struct {
		UserHUID    string `json:"user_huid"`
		GroupChatID string `json:"group_chat_id"`
	} `json:"from"`
}

// Handle runs the configured command, passing payload as JSON on stdin.
// It sets environment variables with callback metadata for the child process.
func (h *ExecHandler) Handle(ctx context.Context, event string, payload []byte) error {
	if h.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, h.timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", h.command)
	cmd.Stdin = bytes.NewReader(payload)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Inherit current environment and add callback metadata.
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "EXPRESS_CALLBACK_EVENT="+event)

	var meta callbackEnvMeta
	if err := json.Unmarshal(payload, &meta); err == nil {
		cmd.Env = append(cmd.Env,
			"EXPRESS_CALLBACK_SYNC_ID="+meta.SyncID,
			"EXPRESS_CALLBACK_BOT_ID="+meta.BotID,
			"EXPRESS_CALLBACK_CHAT_ID="+meta.From.GroupChatID,
			"EXPRESS_CALLBACK_USER_HUID="+meta.From.UserHUID,
		)
	}

	err := cmd.Run()

	// Log stdout at debug level.
	if out := strings.TrimSpace(stdout.String()); out != "" {
		vlog.V2("callback exec %q: stdout: %s", h.command, out)
	}

	// Log stderr as warning.
	stderrStr := strings.TrimSpace(stderr.String())
	if stderrStr != "" {
		vlog.V1("callback exec %q: stderr: %s", h.command, stderrStr)
	}

	if err != nil {
		if ctx.Err() != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("exec handler %q: timeout exceeded, process killed: %w", h.command, ctx.Err())
		}
		return fmt.Errorf("exec handler %q: %w (stderr: %s)", h.command, err, stderrStr)
	}
	return nil
}
