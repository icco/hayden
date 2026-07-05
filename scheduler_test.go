package hayden

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type countingFetcher struct{ n atomic.Int64 }

func (f *countingFetcher) Fetch(_ context.Context, _ *Target) ([]byte, error) {
	f.n.Add(1)
	return []byte("nope"), nil
}

func (f *countingFetcher) count() int { return int(f.n.Load()) }

func TestSchedulerScansOnTickAndStops(t *testing.T) {
	s := NewStore(testDB(t))
	ctx := context.Background()
	if err := s.Create(ctx, &Target{Name: "t", URL: "http://x", MatchType: "substring", MatchValue: "zzz", Enabled: true}); err != nil {
		t.Fatal(err)
	}

	ff := &countingFetcher{}
	sc := &Scanner{Store: s, Notifier: &fakeNotifier{}, Fetcher: ff}
	sch := &Scheduler{
		Scanner:   sc,
		Store:     s,
		Cfg:       &Config{},
		PeriodFor: func(*Target) time.Duration { return 50 * time.Millisecond },
	}

	if err := sch.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for ff.count() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if ff.count() < 2 {
		sch.Stop()
		t.Fatalf("expected >=2 scans, got %d", ff.count())
	}

	sch.Stop()
	after := ff.count()
	time.Sleep(200 * time.Millisecond)
	if ff.count() != after {
		t.Errorf("scans continued after Stop: %d -> %d", after, ff.count())
	}
}
