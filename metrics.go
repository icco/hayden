package hayden

import (
	"context"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type scanMetrics struct {
	scans         metric.Int64Counter
	matches       metric.Int64Counter
	notifications metric.Int64Counter
	duration      metric.Float64Histogram
}

var (
	metricsOnce sync.Once
	metricsInst *scanMetrics
)

// metrics lazily builds the scan instruments from the global meter provider
// (a no-op until the server installs one).
func metrics() *scanMetrics {
	metricsOnce.Do(func() {
		m := otel.Meter("hayden")
		scans, _ := m.Int64Counter("hayden_scans_total")
		matches, _ := m.Int64Counter("hayden_matches_total")
		notes, _ := m.Int64Counter("hayden_notifications_total")
		dur, _ := m.Float64Histogram("hayden_scan_duration_seconds")
		metricsInst = &scanMetrics{scans: scans, matches: matches, notifications: notes, duration: dur}
	})
	return metricsInst
}

func recordScan(ctx context.Context, start time.Time, err error) {
	status := "ok"
	if err != nil {
		status = "error"
	}
	m := metrics()
	m.scans.Add(ctx, 1, metric.WithAttributes(attribute.String("status", status)))
	m.duration.Record(ctx, time.Since(start).Seconds())
}

func recordNotify(ctx context.Context, ok bool) {
	result := "ok"
	if !ok {
		result = "error"
	}
	metrics().notifications.Add(ctx, 1, metric.WithAttributes(attribute.String("result", result)))
}
