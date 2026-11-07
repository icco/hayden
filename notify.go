package hayden

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Event is the JSON payload POSTed to a target's hook when it matches.
type Event struct {
	Target    string    `json:"target"`
	URL       string    `json:"url"`
	Matched   bool      `json:"matched"`
	MatchedAt time.Time `json:"matched_at"`
	MatchType string    `json:"match_type"`
	Match     string    `json:"match,omitempty"` // the matched text, capped at 1000 bytes; empty for inverted targets
}

// Notifier delivers a match event for a target.
type Notifier interface {
	Notify(ctx context.Context, t *Target, ev Event) error
}

// HTTPNotifier POSTs the event JSON to the target hook (or DefaultHook).
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

	var body any = ev
	if isDiscordWebhook(hook) {
		body = discordMessage(ev)
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, hook, bytes.NewReader(buf))
	if err != nil {
		return fmt.Errorf("building webhook request to %s: %w", hookHost(hook), redactURL(err, hook))
	}
	req.Header.Set("Content-Type", "application/json")

	client := n.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("posting webhook to %s: %w", hookHost(hook), redactURL(err, hook))
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("webhook to %s: status %d", hookHost(hook), resp.StatusCode)
	}

	return nil
}

// ShouldNotify decides whether to fire, per the target's NotifyMode:
//   - once (default): fire only on a no-match → match transition.
//   - change: fire whenever the matched text differs from the last.
func ShouldNotify(t *Target, matched bool, contentHash string) bool {
	if !matched {
		return false
	}
	if t.NotifyMode == "change" {
		return contentHash != t.LastContentHash
	}
	return !t.LastMatched
}

// isDiscordWebhook reports whether hook is a Discord webhook URL, which needs a
// Discord message body instead of the raw Event.
func isDiscordWebhook(hook string) bool {
	u, err := url.Parse(hook)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Hostname()) {
	case "discord.com", "discordapp.com", "ptb.discord.com", "canary.discord.com":
		return strings.HasPrefix(u.Path, "/api/webhooks/")
	}
	return false
}

// discordMessage renders ev as a Discord webhook body, with mentions disabled so
// page text like "@everyone" can't ping anyone.
func discordMessage(ev Event) map[string]any {
	content := fmt.Sprintf("%s matched", ev.Target)
	if ev.Match != "" {
		content += ": " + ev.Match
	}
	// Shorten the text, not the link; only an absurdly long URL gets cut.
	content = excerpt(excerpt(content, max(0, 2000-len(ev.URL)-1))+"\n"+ev.URL, 2000)
	return map[string]any{
		"content":          content,
		"allowed_mentions": map[string]any{"parse": []string{}},
	}
}

// hookHost returns only the host of hook; webhook URLs often embed a secret token.
func hookHost(hook string) string {
	if u, err := url.Parse(hook); err == nil && u.Host != "" {
		return u.Host
	}
	return "hook"
}

// redactURL strips the full hook URL from a *url.Error so it can't leak into logs.
func redactURL(err error, hook string) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		ue.URL = hookHost(hook)
	}
	return err
}
