package hayden

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Event is the JSON payload POSTed to a target's hook when it matches.
type Event struct {
	Target    string    `json:"target"`
	URL       string    `json:"url"`
	Matched   bool      `json:"matched"`
	MatchedAt time.Time `json:"matched_at"`
	MatchType string    `json:"match_type"`
}

// Notifier delivers a match event for a target.
type Notifier interface {
	Notify(ctx context.Context, t *Target, ev Event) error
}

// HTTPNotifier POSTs the event as JSON to the target's hook, falling back to
// DefaultHook when the target has none.
type HTTPNotifier struct {
	Client      *http.Client
	DefaultHook string
}

// Notify posts ev to the target's effective hook.
func (n HTTPNotifier) Notify(ctx context.Context, t *Target, ev Event) error {
	hook := t.Hook
	if hook == "" {
		hook = n.DefaultHook
	}
	if hook == "" {
		return fmt.Errorf("no hook configured for target %q", t.Name)
	}

	buf, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("building webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := n.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("posting webhook to %s: %w", hook, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("webhook to %s: status %d", hook, resp.StatusCode)
	}

	return nil
}

// ShouldNotify implements the notify-once rule: fire only on a no-match → match
// transition. The "change" mode arrives in a later phase.
func ShouldNotify(t *Target, matched bool) bool {
	return matched && !t.LastMatched
}
