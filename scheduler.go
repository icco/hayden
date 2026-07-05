package hayden

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Scheduler runs one ticker per enabled target, scanning each on its period.
type Scheduler struct {
	Scanner *Scanner
	Store   *Store
	Cfg     *Config
	Log     *zap.SugaredLogger

	// PeriodFor overrides per-target period resolution (used in tests). When
	// nil, the target's EffectivePeriod is used.
	PeriodFor func(*Target) time.Duration

	mu      sync.Mutex
	base    context.Context //nolint:containedctx // long-lived scan context, re-derived per target on reload
	cancels []context.CancelFunc
	wg      sync.WaitGroup
}

func (s *Scheduler) periodFor(t *Target) time.Duration {
	if s.PeriodFor != nil {
		return s.PeriodFor(t)
	}
	return t.EffectivePeriod(s.Cfg)
}

// Start launches a ticker goroutine for each enabled target. The context bounds
// every scan; cancel it (or call Stop) to shut down.
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
		tctx, cancel := context.WithCancel(ctx)
		s.cancels = append(s.cancels, cancel)
		s.wg.Add(1)
		go s.run(tctx, t.ID, s.periodFor(t))
	}
	return nil
}

func (s *Scheduler) run(ctx context.Context, id uint, period time.Duration) {
	defer s.wg.Done()

	ticker := time.NewTicker(period)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			t, err := s.Store.Get(ctx, id)
			if err != nil {
				s.warn("scheduler: load target", id, err)
				continue
			}
			if !t.Enabled {
				continue
			}
			if err := s.Scanner.Scan(ctx, t); err != nil {
				s.warn("scheduler: scan target", id, err)
			}
		}
	}
}

func (s *Scheduler) warn(msg string, id uint, err error) {
	if s.Log != nil {
		s.Log.Warnw(msg, "target_id", id, "error", err.Error())
	}
}

// Stop cancels every ticker and waits for the goroutines to exit.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	for _, c := range s.cancels {
		c()
	}
	s.cancels = nil
	s.mu.Unlock()
	s.wg.Wait()
}

// Reload restarts the tickers to reflect the current set of enabled targets.
func (s *Scheduler) Reload(ctx context.Context) error {
	s.Stop()
	s.mu.Lock()
	defer s.mu.Unlock()
	base := s.base
	if base == nil {
		base = ctx
	}
	return s.startLocked(base)
}
