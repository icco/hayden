package hayden

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// Scanner runs a single target through fetch → match → notify → persist.
type Scanner struct {
	Store    *Store
	Notifier Notifier

	// Fetcher overrides per-target resolution when set (tests); nil → FetcherFor.
	Fetcher Fetcher
	// Now overrides the clock (tests).
	Now func() time.Time
}

func (sc *Scanner) now() time.Time {
	if sc.Now != nil {
		return sc.Now()
	}
	return time.Now().UTC()
}

func (sc *Scanner) fetcher(t *Target) (Fetcher, error) {
	if sc.Fetcher != nil {
		return sc.Fetcher, nil
	}
	return FetcherFor(t.FetchMode)
}

// Scan runs fetch → match → notify → persist. A failed notify leaves
// LastMatched unchanged so the next tick retries.
func (sc *Scanner) Scan(ctx context.Context, t *Target) error {
	now := sc.now()
	t.LastRunAt = &now

	fetcher, err := sc.fetcher(t)
	if err != nil {
		return sc.fail(ctx, t, err)
	}
	content, err := fetcher.Fetch(ctx, t)
	if err != nil {
		return sc.fail(ctx, t, err)
	}

	matcher, err := MatcherFor(t.MatchType)
	if err != nil {
		return sc.fail(ctx, t, err)
	}
	matched, err := matcher.Match(content, t.MatchValue)
	if err != nil {
		return sc.fail(ctx, t, err)
	}
	if t.Invert {
		matched = !matched
	}

	newHash := hashContent(content)
	t.LastStatus = "ok"
	t.LastError = ""

	if ShouldNotify(t, matched, newHash) {
		ev := Event{Target: t.Name, URL: t.URL, Matched: true, MatchedAt: now, MatchType: t.MatchType}
		if err := sc.Notifier.Notify(ctx, t, ev); err != nil {
			t.LastStatus = "error"
			t.LastError = err.Error()
			// Leave LastMatched/LastContentHash unchanged so the next tick retries.
			_ = sc.Store.SaveRunState(ctx, t)
			return fmt.Errorf("notifying target %q: %w", t.Name, err)
		}
	}

	t.LastContentHash = newHash
	if matched {
		t.LastMatched = true
		t.LastMatchAt = &now
	} else {
		t.LastMatched = false
	}

	return sc.Store.SaveRunState(ctx, t)
}

// ScanAll scans every enabled target, continuing past per-target errors.
func (sc *Scanner) ScanAll(ctx context.Context) error {
	targets, err := sc.Store.ListEnabled(ctx)
	if err != nil {
		return fmt.Errorf("listing targets: %w", err)
	}

	var errs []error
	for _, t := range targets {
		if err := sc.Scan(ctx, t); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// fail records an error run-state and returns the error.
func (sc *Scanner) fail(ctx context.Context, t *Target, err error) error {
	t.LastStatus = "error"
	t.LastError = err.Error()
	_ = sc.Store.SaveRunState(ctx, t)
	return err
}

func hashContent(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
