package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Verify WebhookHandler implements CallbackHandler.
var _ CallbackHandler = (*WebhookHandler)(nil)

func TestWebhookHandler_Type(t *testing.T) {
	h := NewWebhookHandler("http://example.com/hook", 5*time.Second)
	if got := h.Type(); got != "webhook" {
		t.Errorf("Type() = %q, want %q", got, "webhook")
	}
}

func TestWebhookHandler_Handle(t *testing.T) {
	var gotBody string
	var gotHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody = string(body)
		gotHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := NewWebhookHandler(srv.URL, 5*time.Second)
	payload := []byte(`{"sync_id":"sync-123","bot_id":"bot-1"}`)

	err := h.Handle(context.Background(), EventChatCreated, payload)
	if err != nil {
		t.Fatalf("Handle() unexpected error: %v", err)
	}

	if gotBody != string(payload) {
		t.Errorf("body = %q, want %q", gotBody, string(payload))
	}
	if got := gotHeaders.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want %q", got, "application/json")
	}
	if got := gotHeaders.Get("X-Express-Event"); got != EventChatCreated {
		t.Errorf("X-Express-Event = %q, want %q", got, EventChatCreated)
	}
	if got := gotHeaders.Get("X-Express-Sync-ID"); got != "sync-123" {
		t.Errorf("X-Express-Sync-ID = %q, want %q", got, "sync-123")
	}
}

func TestWebhookHandler_NoSyncID(t *testing.T) {
	var gotHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := NewWebhookHandler(srv.URL, 5*time.Second)
	payload := []byte(`{"bot_id":"bot-1"}`)

	err := h.Handle(context.Background(), EventMessage, payload)
	if err != nil {
		t.Fatalf("Handle() unexpected error: %v", err)
	}

	if got := gotHeaders.Get("X-Express-Sync-ID"); got != "" {
		t.Errorf("X-Express-Sync-ID = %q, want empty when no sync_id in payload", got)
	}
}

func TestWebhookHandlerErrors(t *testing.T) {
	t.Run("non-2xx status includes code and body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}))
		defer srv.Close()

		h := NewWebhookHandler(srv.URL, 5*time.Second)

		err := h.Handle(context.Background(), EventMessage, []byte(`{}`))
		if err == nil {
			t.Fatal("Handle() expected error for non-2xx status")
		}
		if !strings.Contains(err.Error(), "500") {
			t.Errorf("expected status code in error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "internal error") {
			t.Errorf("expected response body in error, got: %v", err)
		}
	})

	t.Run("4xx status includes code and body", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("bad request body"))
		}))
		defer srv.Close()

		h := NewWebhookHandler(srv.URL, 5*time.Second)

		err := h.Handle(context.Background(), EventMessage, []byte(`{}`))
		if err == nil {
			t.Fatal("Handle() expected error for 4xx status")
		}
		if !strings.Contains(err.Error(), "400") {
			t.Errorf("expected status code 400 in error, got: %v", err)
		}
	})

	t.Run("response body truncated at 1024 bytes", func(t *testing.T) {
		longBody := strings.Repeat("x", 2048)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(longBody))
		}))
		defer srv.Close()

		h := NewWebhookHandler(srv.URL, 5*time.Second)

		err := h.Handle(context.Background(), EventMessage, []byte(`{}`))
		if err == nil {
			t.Fatal("Handle() expected error")
		}
		// Error message should contain at most 1024 bytes of the body.
		if len(err.Error()) >= len(longBody) {
			t.Errorf("expected truncated body in error, got length %d", len(err.Error()))
		}
	})

	t.Run("timeout returns clear error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		h := NewWebhookHandler(srv.URL, 50*time.Millisecond)

		err := h.Handle(context.Background(), EventMessage, []byte(`{}`))
		if err == nil {
			t.Fatal("Handle() expected error on timeout")
		}
		if !strings.Contains(err.Error(), "timed out") {
			t.Errorf("expected 'timed out' in error, got: %v", err)
		}
	})

	t.Run("connection refused returns clear error", func(t *testing.T) {
		h := NewWebhookHandler("http://127.0.0.1:1", 2*time.Second)

		err := h.Handle(context.Background(), EventMessage, []byte(`{}`))
		if err == nil {
			t.Fatal("Handle() expected error on connection refused")
		}
		if !strings.Contains(err.Error(), "connection refused") {
			t.Errorf("expected 'connection refused' in error, got: %v", err)
		}
	})
}

func TestWebhookHandler_ZeroTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := NewWebhookHandler(srv.URL, 0)

	err := h.Handle(context.Background(), EventMessage, []byte(`{}`))
	if err != nil {
		t.Fatalf("Handle() unexpected error: %v", err)
	}
}

func TestWebhookHandler_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := NewWebhookHandler(srv.URL, 5*time.Second)

	// Invalid JSON — sync_id header should be absent, but request should still work.
	err := h.Handle(context.Background(), EventMessage, []byte(`not json`))
	if err != nil {
		t.Fatalf("Handle() unexpected error: %v", err)
	}
}

func TestWebhookHandler_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	h := NewWebhookHandler(srv.URL, 0)
	err := h.Handle(ctx, EventMessage, []byte(`{}`))
	if err == nil {
		t.Fatal("Handle() expected error on canceled context")
	}
}

