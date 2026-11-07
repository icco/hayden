package hayden

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Scheduler runs one timer per enabled target, scanning each on its period.
type Scheduler struct {
	Scanner *Scanner
	Store   *Store
	Cfg     *Config
	Log     *zap.SugaredLogger

	// PeriodFor overrides per-target period (tests); nil → EffectivePeriod.
	PeriodFor func(*Target) time.Duration
	// Now overrides the clock (tests).
	Now func() time.Time

	mu      sync.Mutex
	base    context.Context //nolint:containedctx // long-lived scan context, outlives each reload
	cancels []context.CancelFunc
	wg      sync.WaitGroup
}

func (s *Scheduler) periodFor(t *Target) time.Duration {
	if s.PeriodFor != nil {
		return s.PeriodFor(t)
	}
	return t.EffectivePeriod(s.Cfg)
}

func (s *Scheduler) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// firstDelay waits out the rest of the target's current period, so restarts and
// reloads neither rescan early nor push the next scan back a full period.
func (s *Scheduler) firstDelay(t *Target, period time.Duration) time.Duration {
	if t.LastRunAt == nil {
		return 0
	}
	return max(0, t.LastRunAt.Add(period).Sub(s.now()))
}

// Start launches a timer per enabled target; cancel ctx or call Stop to end.
// Scans run on ctx, so cancelling it also aborts in-flight scans.
func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.base = ctx
	return s.startLocked(ctx)
}

func (s *Scheduler) startLocked(ctx context.Context) error {
	targets, err := s.Store.ListEnabled(ctx)
	if err != nil {
		return err
	}
	for _, t := range targets {
		tctx, cancel := context.WithCancel(ctx) //nolint:gosec // cancel is retained in s.cancels and invoked by Stop/Reload
		s.cancels = append(s.cancels, cancel)
		period := s.periodFor(t)
		s.wg.Add(1)
		go s.run(tctx, t.ID, period, s.firstDelay(t, period))
	}
	return nil
}

func (s *Scheduler) run(ctx context.Context, id uint, period, delay time.Duration) {
	defer s.wg.Done()

	timer := time.NewTimer(delay)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			// select picks randomly when both are ready; don't start a scan after Stop or Reload.
			if ctx.Err() != nil {
				return
			}
			timer.Reset(s.scan(id, period))
		}
	}
}

// scan runs a due target on the base context, so a reload stops future ticks
// without cancelling a scan that is already running. It reloads the target first,
// so a scan that finished after a reload still counts, and returns the wait until
// the next one, measured from scan start.
func (s *Scheduler) scan(id uint, period time.Duration) time.Duration {
	ctx := s.base //nolint:contextcheck // s.base is the long-lived scan root
	t, err := s.Store.Get(ctx, id)
	if err != nil {
		s.warn("scheduler: load target", id, err)
		return period
	}
	if !t.Enabled {
		return period
	}
	if wait := s.firstDelay(t, period); wait > 0 {
		return wait
	}
	if err := s.Scanner.Scan(ctx, t); err != nil {
		s.warn("scheduler: scan target", id, err)
	}
	// Floor the wait so a scan skipped by the busy guard doesn't spin on a stale last_run_at.
	return max(s.firstDelay(t, period), time.Second)
}

func (s *Scheduler) warn(msg string, id uint, err error) {
	if s.Log != nil {
		s.Log.Warnw(msg, "target_id", id, "error", err.Error())
	}
}

func (s *Scheduler) cancelLocked() {
	for _, c := range s.cancels {
		c()
	}
	s.cancels = nil
}

// Stop cancels every timer and waits for in-flight scans to finish.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	s.cancelLocked()
	s.mu.Unlock()
	s.wg.Wait()
}

// Reload restarts the timers to reflect the current set of enabled targets. It
// does not wait for in-flight scans, so it is safe to call from a request.
func (s *Scheduler) Reload(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cancelLocked()
	if s.base == nil {
		s.base = ctx
	}
	return s.startLocked(s.base) //nolint:contextcheck // s.base is the long-lived scan root
}
