package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lavr/express-botx/internal/config"
)

func TestWithCallbacks(t *testing.T) {
	cfg := config.CallbacksConfig{
		Rules: []config.CallbackRule{
			{
				Events: []string{"chat_created"},
				Async:  false,
				Handler: config.CallbackHandlerConfig{
					Type:    "exec",
					Command: "echo hello",
				},
			},
		},
	}

	sendFn := func(ctx context.Context, p *SendPayload) (string, error) {
		return "test-sync-id", nil
	}
	chatResolver := func(chatID string) (ChatResolveResult, error) {
		return ChatResolveResult{ChatID: chatID}, nil
	}

	srv := New(
		Config{Listen: ":0", BasePath: "/api/v1"},
		sendFn, chatResolver,
		WithCallbacks(cfg),
	)

	if srv.callbackRouter == nil {
		t.Fatal("expected callbackRouter to be set")
	}
	if srv.callbacksCfg == nil {
		t.Fatal("expected callbacksCfg to be set")
	}

	// Verify routing works: chat_created should match, message should not.
	matched := srv.callbackRouter.Route("chat_created")
	if len(matched) != 1 {
		t.Fatalf("expected 1 matched rule for chat_created, got %d", len(matched))
	}
	matched = srv.callbackRouter.Route("message")
	if len(matched) != 0 {
		t.Fatalf("expected 0 matched rules for message, got %d", len(matched))
	}
}

func TestWithCallbacksMultipleRules(t *testing.T) {
	cfg := config.CallbacksConfig{
		Rules: []config.CallbackRule{
			{
				Events: []string{"chat_created", "added_to_chat"},
				Async:  false,
				Handler: config.CallbackHandlerConfig{
					Type:    "exec",
					Command: "echo membership",
				},
			},
			{
				Events: []string{"*"},
				Async:  true,
				Handler: config.CallbackHandlerConfig{
					Type:    "exec",
					Command: "echo fallback",
					Timeout: "5s",
				},
			},
		},
	}

	sendFn := func(ctx context.Context, p *SendPayload) (string, error) {
		return "", nil
	}
	chatResolver := func(chatID string) (ChatResolveResult, error) {
		return ChatResolveResult{ChatID: chatID}, nil
	}

	srv := New(
		Config{Listen: ":0", BasePath: "/api/v1"},
		sendFn, chatResolver,
		WithCallbacks(cfg),
	)

	if srv.callbackRouter == nil {
		t.Fatal("expected callbackRouter to be set")
	}

	// chat_created should match both specific and wildcard rules.
	matched := srv.callbackRouter.Route("chat_created")
	if len(matched) != 2 {
		t.Fatalf("expected 2 matched rules for chat_created, got %d", len(matched))
	}
	if matched[0].async {
		t.Fatal("first rule should be sync")
	}
	if !matched[1].async {
		t.Fatal("second rule should be async")
	}
}

func TestWithCallbackHandler(t *testing.T) {
	customHandler := &recordingHandler{}

	cfg := config.CallbacksConfig{
		Rules: []config.CallbackRule{
			{
				Events: []string{"message"},
				Handler: config.CallbackHandlerConfig{
					Type: "recording", // matches customHandler.Type()
				},
			},
		},
	}

	sendFn := func(ctx context.Context, p *SendPayload) (string, error) {
		return "", nil
	}
	chatResolver := func(chatID string) (ChatResolveResult, error) {
		return ChatResolveResult{ChatID: chatID}, nil
	}

	srv := New(
		Config{Listen: ":0", BasePath: "/api/v1"},
		sendFn, chatResolver,
		WithCallbacks(cfg, WithCallbackHandler(customHandler)),
	)

	if srv.callbackRouter == nil {
		t.Fatal("expected callbackRouter to be set")
	}

	// Test that custom handler is actually used by sending a request.
	body := `{"sync_id":"s1","command":{"body":"hello"},"from":{"group_chat_id":"g1"},"bot_id":"b1"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/command", strings.NewReader(body))
	srv.handleCommand(w, req)

	if w.Code != 202 {
		t.Fatalf("expected 202, got %d", w.Code)
	}
	if customHandler.callCount() != 1 {
		t.Fatalf("expected 1 call to custom handler, got %d", customHandler.callCount())
	}
	if customHandler.lastCall().event != "message" {
		t.Fatalf("expected event 'message', got %q", customHandler.lastCall().event)
	}
}

func TestWithCallbacksNoRules(t *testing.T) {
	cfg := config.CallbacksConfig{
		Rules: []config.CallbackRule{},
	}

	sendFn := func(ctx context.Context, p *SendPayload) (string, error) {
		return "", nil
	}
	chatResolver := func(chatID string) (ChatResolveResult, error) {
		return ChatResolveResult{ChatID: chatID}, nil
	}

	srv := New(
		Config{Listen: ":0", BasePath: "/api/v1"},
		sendFn, chatResolver,
		WithCallbacks(cfg),
	)

	// With no rules, router should still be created (empty).
	if srv.callbackRouter == nil {
		t.Fatal("expected callbackRouter to be set even with no rules")
	}
}

func TestWithCallbacksEndToEnd(t *testing.T) {
	handler := &recordingHandler{}

	cfg := config.CallbacksConfig{
		Rules: []config.CallbackRule{
			{
				Events: []string{"notification_callback"},
				Handler: config.CallbackHandlerConfig{
					Type: "recording",
				},
			},
		},
	}

	sendFn := func(ctx context.Context, p *SendPayload) (string, error) {
		return "", nil
	}
	chatResolver := func(chatID string) (ChatResolveResult, error) {
		return ChatResolveResult{ChatID: chatID}, nil
	}

	srv := New(
		Config{Listen: ":0", BasePath: "/api/v1"},
		sendFn, chatResolver,
		WithCallbacks(cfg, WithCallbackHandler(handler)),
	)

	// Test notification callback endpoint.
	body := `{"sync_id":"n1","status":"ok"}`
	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/notification/callback", strings.NewReader(body))
	srv.handleNotificationCallback(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp callbackResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Result != "ok" {
		t.Fatalf("expected result 'ok', got %q", resp.Result)
	}

	if handler.callCount() != 1 {
		t.Fatalf("expected 1 call, got %d", handler.callCount())
	}
}
