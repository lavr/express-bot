package server

import (
	"context"
	"testing"

	"github.com/lavr/express-botx/internal/config"
)

// stubHandler is a minimal CallbackHandler for testing routing logic.
type stubHandler struct {
	name string
}

func (s *stubHandler) Type() string { return "stub" }
func (s *stubHandler) Handle(_ context.Context, _ string, _ []byte) error {
	return nil
}

func TestCallbackRouter(t *testing.T) {
	h1 := &stubHandler{name: "h1"}
	h2 := &stubHandler{name: "h2"}
	h3 := &stubHandler{name: "h3"}

	router, err := NewCallbackRouter(
		[][]string{
			{"chat_created", "added_to_chat"},
			{"cts_login", "cts_logout"},
			{"*"},
		},
		[]bool{false, true, true},
		map[int]CallbackHandler{0: h1, 1: h2, 2: h3},
	)
	if err != nil {
		t.Fatalf("NewCallbackRouter: %v", err)
	}

	t.Run("exact match single rule", func(t *testing.T) {
		matched := router.Route("cts_login")
		if len(matched) != 2 {
			t.Fatalf("expected 2 matched rules, got %d", len(matched))
		}
		if matched[0].handler != h2 {
			t.Errorf("first match should be h2")
		}
		if matched[0].async != true {
			t.Errorf("first match should be async")
		}
		// wildcard also matches
		if matched[1].handler != h3 {
			t.Errorf("second match should be h3 (wildcard)")
		}
	})

	t.Run("exact match multiple events in rule", func(t *testing.T) {
		matched := router.Route("added_to_chat")
		if len(matched) != 2 {
			t.Fatalf("expected 2 matched rules, got %d", len(matched))
		}
		if matched[0].handler != h1 {
			t.Errorf("first match should be h1")
		}
		if matched[0].async != false {
			t.Errorf("first match should be sync")
		}
	})

	t.Run("wildcard only match", func(t *testing.T) {
		matched := router.Route("unknown_event")
		if len(matched) != 1 {
			t.Fatalf("expected 1 matched rule (wildcard), got %d", len(matched))
		}
		if matched[0].handler != h3 {
			t.Errorf("match should be h3 (wildcard)")
		}
		if matched[0].async != true {
			t.Errorf("wildcard match should be async")
		}
	})

	t.Run("all matching rules returned in order", func(t *testing.T) {
		matched := router.Route("chat_created")
		if len(matched) != 2 {
			t.Fatalf("expected 2 matched rules, got %d", len(matched))
		}
		if matched[0].handler != h1 {
			t.Errorf("first match should be h1")
		}
		if matched[1].handler != h3 {
			t.Errorf("second match should be h3 (wildcard)")
		}
	})
}

func TestCallbackRouterNoWildcard(t *testing.T) {
	h1 := &stubHandler{name: "h1"}

	router, err := NewCallbackRouter(
		[][]string{{"chat_created"}},
		[]bool{false},
		map[int]CallbackHandler{0: h1},
	)
	if err != nil {
		t.Fatalf("NewCallbackRouter: %v", err)
	}

	matched := router.Route("cts_login")
	if len(matched) != 0 {
		t.Errorf("expected no matches for unmatched event, got %d", len(matched))
	}
}

func TestCallbackRouterEmpty(t *testing.T) {
	router, err := NewCallbackRouter(nil, nil, map[int]CallbackHandler{})
	if err != nil {
		t.Fatalf("NewCallbackRouter: %v", err)
	}

	matched := router.Route("anything")
	if len(matched) != 0 {
		t.Errorf("expected no matches for empty router, got %d", len(matched))
	}
}

func TestBuildHandlers(t *testing.T) {
	t.Run("exec handler", func(t *testing.T) {
		rules := []config.CallbackRule{
			{
				Events: []string{"chat_created"},
				Handler: config.CallbackHandlerConfig{
					Type:    "exec",
					Command: "./handler.sh",
					Timeout: "5s",
				},
			},
		}
		handlers, err := buildHandlers(rules, nil)
		if err != nil {
			t.Fatalf("buildHandlers: %v", err)
		}
		if len(handlers) != 1 {
			t.Fatalf("expected 1 handler, got %d", len(handlers))
		}
		if handlers[0].Type() != "exec" {
			t.Errorf("expected exec handler, got %s", handlers[0].Type())
		}
	})

	t.Run("webhook handler", func(t *testing.T) {
		rules := []config.CallbackRule{
			{
				Events: []string{"cts_login"},
				Handler: config.CallbackHandlerConfig{
					Type:    "webhook",
					URL:     "http://example.com/hook",
					Timeout: "30s",
				},
			},
		}
		handlers, err := buildHandlers(rules, nil)
		if err != nil {
			t.Fatalf("buildHandlers: %v", err)
		}
		if len(handlers) != 1 {
			t.Fatalf("expected 1 handler, got %d", len(handlers))
		}
		if handlers[0].Type() != "webhook" {
			t.Errorf("expected webhook handler, got %s", handlers[0].Type())
		}
	})

	t.Run("multiple rules mixed types", func(t *testing.T) {
		rules := []config.CallbackRule{
			{
				Events:  []string{"chat_created"},
				Handler: config.CallbackHandlerConfig{Type: "exec", Command: "./h.sh"},
			},
			{
				Events:  []string{"cts_login"},
				Handler: config.CallbackHandlerConfig{Type: "webhook", URL: "http://x.com"},
			},
		}
		handlers, err := buildHandlers(rules, nil)
		if err != nil {
			t.Fatalf("buildHandlers: %v", err)
		}
		if len(handlers) != 2 {
			t.Fatalf("expected 2 handlers, got %d", len(handlers))
		}
		if handlers[0].Type() != "exec" {
			t.Errorf("handler 0: expected exec, got %s", handlers[0].Type())
		}
		if handlers[1].Type() != "webhook" {
			t.Errorf("handler 1: expected webhook, got %s", handlers[1].Type())
		}
	})

	t.Run("no timeout", func(t *testing.T) {
		rules := []config.CallbackRule{
			{
				Events:  []string{"message"},
				Handler: config.CallbackHandlerConfig{Type: "exec", Command: "./h.sh"},
			},
		}
		handlers, err := buildHandlers(rules, nil)
		if err != nil {
			t.Fatalf("buildHandlers: %v", err)
		}
		if handlers[0].Type() != "exec" {
			t.Errorf("expected exec handler, got %s", handlers[0].Type())
		}
	})

	t.Run("invalid timeout", func(t *testing.T) {
		rules := []config.CallbackRule{
			{
				Events:  []string{"message"},
				Handler: config.CallbackHandlerConfig{Type: "exec", Command: "./h.sh", Timeout: "bad"},
			},
		}
		_, err := buildHandlers(rules, nil)
		if err == nil {
			t.Fatal("expected error for invalid timeout")
		}
	})

	t.Run("unknown handler type", func(t *testing.T) {
		rules := []config.CallbackRule{
			{
				Events:  []string{"message"},
				Handler: config.CallbackHandlerConfig{Type: "grpc"},
			},
		}
		_, err := buildHandlers(rules, nil)
		if err == nil {
			t.Fatal("expected error for unknown handler type")
		}
	})

	t.Run("custom handler from registry", func(t *testing.T) {
		custom := map[string]CallbackHandler{
			"custom": &stubHandler{name: "my-custom"},
		}
		rules := []config.CallbackRule{
			{
				Events:  []string{"message"},
				Handler: config.CallbackHandlerConfig{Type: "custom"},
			},
		}
		handlers, err := buildHandlers(rules, custom)
		if err != nil {
			t.Fatalf("buildHandlers: %v", err)
		}
		if handlers[0].Type() != "stub" {
			t.Errorf("expected stub (custom) handler, got %s", handlers[0].Type())
		}
	})

	t.Run("custom handler overrides built-in type", func(t *testing.T) {
		custom := map[string]CallbackHandler{
			"exec": &stubHandler{name: "custom-exec"},
		}
		rules := []config.CallbackRule{
			{
				Events:  []string{"message"},
				Handler: config.CallbackHandlerConfig{Type: "exec", Command: "./h.sh"},
			},
		}
		handlers, err := buildHandlers(rules, custom)
		if err != nil {
			t.Fatalf("buildHandlers: %v", err)
		}
		// Custom handler should take precedence.
		if handlers[0].Type() != "stub" {
			t.Errorf("expected custom handler to override exec, got %s", handlers[0].Type())
		}
	})

	t.Run("empty rules", func(t *testing.T) {
		handlers, err := buildHandlers(nil, nil)
		if err != nil {
			t.Fatalf("buildHandlers: %v", err)
		}
		if len(handlers) != 0 {
			t.Errorf("expected 0 handlers, got %d", len(handlers))
		}
	})
}
