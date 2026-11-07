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

func TestSchedulerFirstDelay(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s := &Scheduler{Now: func() time.Time { return now }}
	ago := func(d time.Duration) *time.Time { v := now.Add(-d); return &v }

	for _, tc := range []struct {
		name    string
		lastRun *time.Time
		want    time.Duration
	}{
		{"never run", nil, 0},
		{"mid period", ago(2 * time.Minute), 3 * time.Minute},
		{"overdue", ago(time.Hour), 0},
	} {
		if got := s.firstDelay(&Target{LastRunAt: tc.lastRun}, 5*time.Minute); got != tc.want {
			t.Errorf("%s: firstDelay = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestSchedulerScanSkipsTargetNotDue(t *testing.T) {
	s := NewStore(testDB(t))
	ctx := context.Background()
	recent := time.Now().Add(-time.Minute)
	tg := &Target{Name: "t", URL: "http://x", MatchType: "substring", MatchValue: "zzz", Enabled: true}
	if err := s.Create(ctx, tg); err != nil {
		t.Fatal(err)
	}
	tg.LastRunAt = &recent
	if err := s.SaveRunState(ctx, tg); err != nil {
		t.Fatal(err)
	}

	ff := &countingFetcher{}
	sch := &Scheduler{Scanner: &Scanner{Store: s, Notifier: &fakeNotifier{}, Fetcher: ff}, Store: s, base: ctx}
	wait := sch.scan(tg.ID, 5*time.Minute)
	if ff.count() != 0 {
		t.Errorf("scanned a target that ran a minute ago")
	}
	if wait < 3*time.Minute || wait > 4*time.Minute {
		t.Errorf("wait = %v, want about 4m", wait)
	}
}
