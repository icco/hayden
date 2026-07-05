package hayden

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeFetcher struct {
	content []byte
	err     error
}

func (f *fakeFetcher) Fetch(_ context.Context, _ *Target) ([]byte, error) {
	return f.content, f.err
}

type fakeNotifier struct {
	calls  int
	lastEv Event
}

func (f *fakeNotifier) Notify(_ context.Context, _ *Target, ev Event) error {
	f.calls++
	f.lastEv = ev
	return nil
}

func TestScanMatchNotifiesOnce(t *testing.T) {
	s := NewStore(testDB(t))
	ctx := context.Background()
	tg := &Target{Name: "t", URL: "http://x", MatchType: "substring", MatchValue: "boom", FetchMode: "http", NotifyMode: "once", Enabled: true}
	if err := s.Create(ctx, tg); err != nil {
		t.Fatalf("create: %v", err)
	}

	fn := &fakeNotifier{}
	sc := &Scanner{
		Store:    s,
		Notifier: fn,
		Fetcher:  &fakeFetcher{content: []byte("kaboom!")},
		Now:      func() time.Time { return time.Unix(1000, 0).UTC() },
	}

	if err := sc.Scan(ctx, tg); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if fn.calls != 1 {
		t.Fatalf("notify calls = %d, want 1", fn.calls)
	}
	if fn.lastEv.Target != "t" || !fn.lastEv.Matched {
		t.Errorf("event = %+v", fn.lastEv)
	}

	got, err := s.Get(ctx, tg.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.LastMatched || got.LastStatus != "ok" || got.LastMatchAt == nil || got.LastContentHash == "" {
		t.Errorf("run-state after match: %+v", got)
	}

	// Second scan, still matching → notify-once must not fire again.
	if err := sc.Scan(ctx, got); err != nil {
		t.Fatalf("second scan: %v", err)
	}
	if fn.calls != 1 {
		t.Errorf("notify calls after 2nd scan = %d, want 1", fn.calls)
	}
}

func TestScanFetchError(t *testing.T) {
	s := NewStore(testDB(t))
	ctx := context.Background()
	tg := &Target{Name: "e", URL: "http://x", MatchType: "substring", MatchValue: "x", Enabled: true}
	if err := s.Create(ctx, tg); err != nil {
		t.Fatal(err)
	}

	fn := &fakeNotifier{}
	sc := &Scanner{Store: s, Notifier: fn, Fetcher: &fakeFetcher{err: errors.New("boom")}}

	if err := sc.Scan(ctx, tg); err == nil {
		t.Error("expected error from fetch failure")
	}
	if fn.calls != 0 {
		t.Errorf("must not notify on fetch error, calls=%d", fn.calls)
	}
	got, _ := s.Get(ctx, tg.ID)
	if got.LastStatus != "error" || got.LastError == "" {
		t.Errorf("error run-state not persisted: %+v", got)
	}
}

func TestScanInvertNotifiesWhenAbsent(t *testing.T) {
	s := NewStore(testDB(t))
	ctx := context.Background()
	tg := &Target{Name: "inv", URL: "http://x", MatchType: "substring", MatchValue: "in stock", Invert: true, Enabled: true}
	if err := s.Create(ctx, tg); err != nil {
		t.Fatal(err)
	}

	fn := &fakeNotifier{}
	sc := &Scanner{Store: s, Notifier: fn, Fetcher: &fakeFetcher{content: []byte("out of stock")}}

	if err := sc.Scan(ctx, tg); err != nil {
		t.Fatal(err)
	}
	if fn.calls != 1 {
		t.Errorf("invert: expected notify when value absent, calls=%d", fn.calls)
	}
}

func TestScanAllContinuesPastErrors(t *testing.T) {
	s := NewStore(testDB(t))
	ctx := context.Background()
	if err := s.Create(ctx, &Target{Name: "ok", URL: "http://x", MatchType: "substring", MatchValue: "hit", Enabled: true}); err != nil {
		t.Fatal(err)
	}

	fn := &fakeNotifier{}
	sc := &Scanner{Store: s, Notifier: fn, Fetcher: &fakeFetcher{content: []byte("a hit here")}}
	if err := sc.ScanAll(ctx); err != nil {
		t.Fatalf("scan all: %v", err)
	}
	if fn.calls != 1 {
		t.Errorf("expected 1 notification, got %d", fn.calls)
	}
}
