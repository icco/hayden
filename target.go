package hayden

import (
	"context"
	"fmt"
	"net/url"
)

// Target is a single web page to watch and the match that triggers its hook.
type Target struct {
	URL    string `json:"url"`
	Text   string `json:"text"`
	Invert bool   `json:"invert"`
	Hook   string `json:"hook,omitempty"`
	Period int    `json:"period,omitempty"`
}

// Scan fetches the target and reports whether it matches, honoring Invert.
func (t *Target) Scan(ctx context.Context, cfg *Config) (bool, error) {
	u, err := url.Parse(t.URL)
	if err != nil {
		return false, fmt.Errorf("bad target URL %q: %w", t.URL, err)
	}

	found, err := cfg.Find(ctx, u, t.Text)
	if err != nil {
		return false, err
	}

	if t.Invert {
		return !found, nil
	}

	return found, nil
}
