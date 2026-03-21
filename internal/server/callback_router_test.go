package server

import (
	"context"
	"testing"
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
