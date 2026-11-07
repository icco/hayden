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

func TestScanNotifyModeChange(t *testing.T) {
	s := NewStore(testDB(t))
	ctx := context.Background()
	tg := &Target{Name: "c", URL: "http://x", MatchType: "regex", MatchValue: `hit v\d`, NotifyMode: "change", Enabled: true}
	if err := s.Create(ctx, tg); err != nil {
		t.Fatal(err)
	}

	fn := &fakeNotifier{}
	ff := &fakeFetcher{content: []byte("a hit v1")}
	sc := &Scanner{Store: s, Notifier: fn, Fetcher: ff}

	// First match → notify.
	if err := sc.Scan(ctx, tg); err != nil {
		t.Fatal(err)
	}
	if fn.calls != 1 {
		t.Fatalf("first scan calls = %d, want 1", fn.calls)
	}

	// Page changes outside the match → no re-notify.
	ff.content = []byte("ad 2 a hit v1")
	got, _ := s.Get(ctx, tg.ID)
	if err := sc.Scan(ctx, got); err != nil {
		t.Fatal(err)
	}
	if fn.calls != 1 {
		t.Fatalf("unchanged content calls = %d, want 1", fn.calls)
	}

	// Changed content, still matching → notify again.
	ff.content = []byte("a hit v2")
	got, _ = s.Get(ctx, tg.ID)
	if err := sc.Scan(ctx, got); err != nil {
		t.Fatal(err)
	}
	if fn.calls != 2 {
		t.Fatalf("changed content calls = %d, want 2", fn.calls)
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

func TestScanRecordsHistoryAndSnippet(t *testing.T) {
	s := NewStore(testDB(t))
	ctx := context.Background()
	tg := &Target{Name: "h", URL: "http://x", MatchType: "regex", MatchValue: `\$\d+`, Enabled: true}
	if err := s.Create(ctx, tg); err != nil {
		t.Fatal(err)
	}

	fn := &fakeNotifier{}
	ff := &fakeFetcher{content: []byte("now $499")}
	sc := &Scanner{Store: s, Notifier: fn, Fetcher: ff}
	if err := sc.Scan(ctx, tg); err != nil {
		t.Fatal(err)
	}
	if fn.lastEv.Match != "$499" {
		t.Errorf("event match = %q, want $499", fn.lastEv.Match)
	}

	ff.err = errors.New("boom")
	_ = sc.Scan(ctx, tg)

	scans, err := s.ListScans(ctx, tg.ID, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(scans) != 2 {
		t.Fatalf("history len = %d, want 2", len(scans))
	}
	if scans[0].Status != "error" || scans[0].Error == "" {
		t.Errorf("newest scan = %+v, want error", scans[0])
	}
	if scans[1].Status != "ok" || !scans[1].Matched || !scans[1].Notified || scans[1].Match != "$499" {
		t.Errorf("oldest scan = %+v, want notified match", scans[1])
	}
}

func TestRecordScanPrunes(t *testing.T) {
	s := NewStore(testDB(t))
	ctx := context.Background()
	for range historyLimit + 5 {
		if err := s.RecordScan(ctx, &ScanRecord{TargetID: 7, Status: "ok"}); err != nil {
			t.Fatal(err)
		}
	}
	scans, err := s.ListScans(ctx, 7, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(scans) != historyLimit {
		t.Errorf("history len = %d, want %d", len(scans), historyLimit)
	}
}
