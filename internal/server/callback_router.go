package server

import (
	"fmt"
	"time"

	"github.com/lavr/express-botx/internal/config"
)

// matchedRule pairs a handler with its async flag from the matching rule.
type matchedRule struct {
	handler CallbackHandler
	async   bool
}

// routerRule is an internal representation of a callback routing rule.
type routerRule struct {
	events  []string
	handler CallbackHandler
	async   bool
}

// CallbackRouter matches callback events to their configured handlers.
type CallbackRouter struct {
	rules []routerRule
}

// NewCallbackRouter creates a CallbackRouter from config rules and a handler registry.
// handlerRegistry maps rule index to a CallbackHandler instance.
func NewCallbackRouter(events [][]string, asyncFlags []bool, handlers map[int]CallbackHandler) (*CallbackRouter, error) {
	if len(events) != len(asyncFlags) {
		return nil, fmt.Errorf("callback router: events and asyncFlags length mismatch")
	}

	rules := make([]routerRule, 0, len(events))
	for i, evts := range events {
		h, ok := handlers[i]
		if !ok {
			return nil, fmt.Errorf("callback router: no handler for rule %d", i)
		}
		if len(evts) == 0 {
			return nil, fmt.Errorf("callback router: rule %d has no events", i)
		}
		rules = append(rules, routerRule{
			events:  evts,
			handler: h,
			async:   asyncFlags[i],
		})
	}

	return &CallbackRouter{rules: rules}, nil
}

// buildHandlers creates a CallbackHandler for each config rule, returning a map
// keyed by rule index. Custom handlers from the registry take precedence: if a
// rule's handler type matches a registered custom handler, that handler is used
// instead of the built-in exec/webhook constructors.
func buildHandlers(rules []config.CallbackRule, custom map[string]CallbackHandler) (map[int]CallbackHandler, error) {
	handlers := make(map[int]CallbackHandler, len(rules))
	for i, rule := range rules {
		var timeout time.Duration
		if rule.Handler.Timeout != "" {
			var err error
			timeout, err = time.ParseDuration(rule.Handler.Timeout)
			if err != nil {
				return nil, fmt.Errorf("callback rule #%d: invalid timeout %q: %w", i+1, rule.Handler.Timeout, err)
			}
		}

		// Check custom handler registry first.
		if custom != nil {
			if ch, ok := custom[rule.Handler.Type]; ok {
				handlers[i] = ch
				continue
			}
		}

		switch rule.Handler.Type {
		case "exec":
			handlers[i] = NewExecHandler(rule.Handler.Command, timeout)
		case "webhook":
			handlers[i] = NewWebhookHandler(rule.Handler.URL, timeout)
		default:
			return nil, fmt.Errorf("callback rule #%d: unknown handler type %q", i+1, rule.Handler.Type)
		}
	}
	return handlers, nil
}

// Route returns all rules that match the given event, in declaration order.
// A rule matches if its events list contains the exact event name or the wildcard "*".
func (r *CallbackRouter) Route(event string) []matchedRule {
	var matched []matchedRule
	for _, rule := range r.rules {
		for _, e := range rule.events {
			if e == event || e == "*" {
				matched = append(matched, matchedRule{
					handler: rule.handler,
					async:   rule.async,
				})
				break
			}
		}
	}
	return matched
}
