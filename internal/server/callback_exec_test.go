package server

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Verify ExecHandler implements CallbackHandler.
var _ CallbackHandler = (*ExecHandler)(nil)

func TestExecHandler_Type(t *testing.T) {
	h := NewExecHandler("echo hello", 5*time.Second)
	if got := h.Type(); got != "exec" {
		t.Errorf("Type() = %q, want %q", got, "exec")
	}
}

func TestExecHandler_Handle(t *testing.T) {
	h := NewExecHandler("cat > /dev/null", 5*time.Second)
	payload := []byte(`{"sync_id":"test-123","bot_id":"bot-1"}`)

	err := h.Handle(context.Background(), EventMessage, payload)
	if err != nil {
		t.Fatalf("Handle() unexpected error: %v", err)
	}
}

func TestExecHandler_StdinPayload(t *testing.T) {
	// Verify the command receives the payload on stdin by using grep to check content.
	h := NewExecHandler(`grep -q "sync_id"`, 5*time.Second)
	payload := []byte(`{"sync_id":"test-456"}`)

	err := h.Handle(context.Background(), EventMessage, payload)
	if err != nil {
		t.Fatalf("Handle() error: %v; expected payload to be passed on stdin", err)
	}
}

func TestExecHandler_NonZeroExit(t *testing.T) {
	h := NewExecHandler("exit 1", 5*time.Second)

	err := h.Handle(context.Background(), EventMessage, []byte(`{}`))
	if err == nil {
		t.Fatal("Handle() expected error for non-zero exit code")
	}
}

func TestExecHandler_Timeout(t *testing.T) {
	h := NewExecHandler("sleep 10", 100*time.Millisecond)

	err := h.Handle(context.Background(), EventMessage, []byte(`{}`))
	if err == nil {
		t.Fatal("Handle() expected error on timeout")
	}
}

func TestExecHandler_EnvVariables(t *testing.T) {
	// The command prints env vars so we can verify them.
	h := NewExecHandler(`env | grep EXPRESS_CALLBACK_`, 5*time.Second)

	payload := []byte(`{
		"sync_id": "sync-abc-123",
		"bot_id": "bot-uuid-456",
		"from": {
			"user_huid": "user-huid-789",
			"group_chat_id": "chat-id-012"
		}
	}`)

	// Capture stdout to verify env vars.
	h2 := NewExecHandler(
		`test "$EXPRESS_CALLBACK_EVENT" = "chat_created" && `+
			`test "$EXPRESS_CALLBACK_SYNC_ID" = "sync-abc-123" && `+
			`test "$EXPRESS_CALLBACK_BOT_ID" = "bot-uuid-456" && `+
			`test "$EXPRESS_CALLBACK_CHAT_ID" = "chat-id-012" && `+
			`test "$EXPRESS_CALLBACK_USER_HUID" = "user-huid-789"`,
		5*time.Second,
	)

	err := h2.Handle(context.Background(), EventChatCreated, payload)
	if err != nil {
		t.Fatalf("Handle() error: %v; expected all env vars to be set correctly", err)
	}

	// Also verify h doesn't error (basic smoke test).
	err = h.Handle(context.Background(), EventMessage, payload)
	if err != nil {
		t.Fatalf("Handle() unexpected error: %v", err)
	}
}

func TestExecHandler_EnvVariablesPartialPayload(t *testing.T) {
	// When payload has only some fields, missing ones should be empty strings.
	h := NewExecHandler(
		`test "$EXPRESS_CALLBACK_EVENT" = "notification_callback" && `+
			`test "$EXPRESS_CALLBACK_SYNC_ID" = "sync-only" && `+
			`test "$EXPRESS_CALLBACK_BOT_ID" = "" && `+
			`test "$EXPRESS_CALLBACK_USER_HUID" = ""`,
		5*time.Second,
	)

	payload := []byte(`{"sync_id": "sync-only"}`)

	err := h.Handle(context.Background(), EventNotificationCallback, payload)
	if err != nil {
		t.Fatalf("Handle() error: %v; expected partial payload to work with empty env vars", err)
	}
}

func TestExecHandler_EnvVariablesInvalidJSON(t *testing.T) {
	// Invalid JSON should still work — env vars just won't include metadata.
	h := NewExecHandler(
		`test "$EXPRESS_CALLBACK_EVENT" = "message"`,
		5*time.Second,
	)

	err := h.Handle(context.Background(), EventMessage, []byte(`not json`))
	if err != nil {
		t.Fatalf("Handle() error: %v; expected invalid JSON to be handled gracefully", err)
	}
}

func TestExecHandler_ZeroTimeout(t *testing.T) {
	// Zero timeout means no deadline is applied.
	h := NewExecHandler("echo ok", 0)

	err := h.Handle(context.Background(), EventMessage, []byte(`{}`))
	if err != nil {
		t.Fatalf("Handle() unexpected error: %v", err)
	}
}

func TestExecHandlerTimeout(t *testing.T) {
	h := NewExecHandler("sleep 10", 100*time.Millisecond)

	err := h.Handle(context.Background(), EventMessage, []byte(`{}`))
	if err == nil {
		t.Fatal("Handle() expected error on timeout")
	}
	if !strings.Contains(err.Error(), "timeout exceeded") {
		t.Errorf("expected timeout error message, got: %v", err)
	}
	if !strings.Contains(err.Error(), "process killed") {
		t.Errorf("expected 'process killed' in error message, got: %v", err)
	}
}

func TestExecHandlerTimeout_StderrCapture(t *testing.T) {
	// Command that writes to stderr before hanging.
	h := NewExecHandler(`echo "error output" >&2; exit 1`, 5*time.Second)

	err := h.Handle(context.Background(), EventMessage, []byte(`{}`))
	if err == nil {
		t.Fatal("Handle() expected error")
	}
	if !strings.Contains(err.Error(), "error output") {
		t.Errorf("expected stderr in error, got: %v", err)
	}
}

func TestExecHandlerTimeout_StdoutCapture(t *testing.T) {
	// Command that writes to stdout — should succeed, stdout logged at debug level.
	h := NewExecHandler(`echo "hello from handler"`, 5*time.Second)

	err := h.Handle(context.Background(), EventMessage, []byte(`{}`))
	if err != nil {
		t.Fatalf("Handle() unexpected error: %v", err)
	}
}

func TestExecHandlerTimeout_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	h := NewExecHandler("sleep 10", 0)
	err := h.Handle(ctx, EventMessage, []byte(`{}`))
	if err == nil {
		t.Fatal("Handle() expected error on canceled context")
	}
}
