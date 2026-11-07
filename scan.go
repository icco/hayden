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

// Scan runs fetch → match → notify → persist and appends the outcome to the
// target's history. A failed notify leaves LastMatched unchanged so the next
// tick retries.
func (sc *Scanner) Scan(ctx context.Context, t *Target) (err error) {
	start := time.Now()
	rec := &ScanRecord{TargetID: t.ID}
	defer func() {
		recordScan(ctx, start, err)
		rec.Status, rec.Error = "ok", ""
		if err != nil {
			rec.Status, rec.Error = "error", err.Error()
		}
		if herr := sc.Store.RecordScan(ctx, rec); herr != nil {
			err = errors.Join(err, fmt.Errorf("recording scan history: %w", herr))
		}
	}()

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
	snippet, matched, err := matcher.Find(content, t.MatchValue)
	if err != nil {
		return sc.fail(ctx, t, err)
	}
	if t.Invert {
		matched, snippet = !matched, ""
	}
	snippet = truncate(snippet, maxSnippet)
	rec.Matched, rec.Match = matched, snippet
	if matched {
		metrics().matches.Add(ctx, 1)
	}

	newHash := hashContent([]byte(snippet))
	t.LastStatus = "ok"
	t.LastError = ""

	if ShouldNotify(t, matched, newHash) {
		ev := Event{Target: t.Name, URL: t.URL, Matched: true, MatchedAt: now, MatchType: t.MatchType, Match: snippet}
		if nerr := sc.Notifier.Notify(ctx, t, ev); nerr != nil {
			recordNotify(ctx, false)
			// Leave LastMatched/LastContentHash unchanged so the next tick retries.
			return sc.fail(ctx, t, fmt.Errorf("notifying target %q: %w", t.Name, nerr))
		}
		recordNotify(ctx, true)
		rec.Notified = true
	}

	t.LastContentHash = newHash
	t.LastMatched = matched
	if matched {
		t.LastMatchAt = &now
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
	if serr := sc.Store.SaveRunState(ctx, t); serr != nil {
		return errors.Join(err, fmt.Errorf("saving run state: %w", serr))
	}
	return err
}

func hashContent(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
