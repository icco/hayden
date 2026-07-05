# Hayden Phase 4+5 (Matching + Observability) Implementation Plan

> REQUIRED SUB-SKILL: superpowers:executing-plans. TDD. One branch → one PR.

**Goal:** Broaden matching (css/regex/jsonpath + `notify_mode: change`) and add observability (`/metrics`, target-list `/`).

**Tech Stack:** goquery (css), stdlib regexp, tidwall/gjson (jsonpath), prometheus + OTel (reportd pattern), otelhttp.

## Global Constraints

- Keep Postgres-only, `DATABASE_URL`, `HAYDEN_API_TOKEN`, `CGO_ENABLED=0`, single container.
- Pure-logic tests (matchers, notify decision) need no DB; store/scan tests use `TEST_DATABASE_URL`.
- `/metrics` must not require the API token; exclude it from otelhttp span/metric noise.
- Never force-push.

## Interfaces (locked)

```go
// match.go — Matcher gains Validate so POST /targets can reject bad values early.
type Matcher interface {
    Match(content []byte, value string) (bool, error)
    Validate(value string) error
}
// matchers: substring (done), css (goquery), regex (regexp), jsonpath (gjson)

// notify.go — mode-aware decision (adds change).
func ShouldNotify(t *Target, matched bool, contentHash string) bool
// once:   matched && !t.LastMatched
// change: matched && contentHash != t.LastContentHash
```

---

### Task 1: Matcher.Validate + css/regex/jsonpath matchers

**Files:** `match.go`, `match_test.go`; deps goquery, gjson.

- css: `match_value` is a CSS selector; match if goquery selection is non-empty. Validate parses the selector.
- regex: `regexp.Compile(value)`; match if it matches content. Validate compiles.
- jsonpath: `gjson.GetBytes(content, value)`; match if result Exists() and is not `false`/`null`. Validate checks `gjson.Valid`-style path (gjson paths don't pre-compile; Validate = non-empty path).
- substring/`""`: Validate → nil.

- [ ] Write table tests: each matcher's Match true/false cases + Validate rejects a bad selector / bad regex.
- [ ] Run → FAIL.
- [ ] Implement matchers + add `Validate` to all (incl. substring). `MatcherFor` returns each.
- [ ] Run → PASS. Commit: `feat: add css, regex, and jsonpath matchers`

### Task 2: notify_mode "change"

**Files:** `notify.go`, `notify_test.go`, `scan.go`, `scan_test.go`.

- [ ] Update `ShouldNotify` signature + `change` branch; update once tests; add change tests (matched+hash changed → notify; matched+same hash → no; not matched → no).
- [ ] In `scan.go`: compute `newHash` into a local, call `ShouldNotify(t, matched, newHash)` (compares vs old `t.LastContentHash`), THEN set `t.LastContentHash = newHash`. Add a scan test: `NotifyMode:"change"` re-notifies when content changes.
- [ ] Run → PASS. Commit: `feat: add notify_mode change`

### Task 3: validate match_value in the API

**Files:** `server/server.go`.

- [ ] In `POST /targets`, after resolving `MatcherFor(t.MatchType)`, call `m.Validate(t.MatchValue)` and 400 on error.
- [ ] Manual: `POST` a bad regex → 400. Commit: `feat: validate match_value on create`

### Task 4: scan metrics instruments

**Files:** `metrics.go` (new), `scan.go`, `scan_test.go`; deps otel.

- [ ] `metrics.go`: build counters/histogram from `otel.Meter("hayden")` once (sync.Once): `hayden_scans_total{status}`, `hayden_matches_total`, `hayden_notifications_total{result}`, `hayden_scan_duration_seconds`. Safe with the global noop provider when unset.
- [ ] Record them in `scan.go` (duration around the scan; status ok/error; match; notify result). Keep the Scanner working without a provider (tests unaffected).
- [ ] Run existing scan tests → PASS (behavior unchanged). Commit: `feat: instrument scans with otel metrics`

### Task 5: server /metrics + otelhttp + meter provider

**Files:** `server/server.go`; deps prometheus, otelprom, sdkmetric, otelhttp.

- [ ] Wire reportd's meter provider (prometheus registry → otelprom exporter → sdkmetric MeterProvider → `otel.SetMeterProvider`; defer Shutdown).
- [ ] Mount `GET /metrics` = `promhttp.HandlerFor(registry, ...)` (public, no token).
- [ ] Wrap the router with `otelhttp.NewHandler(r, "hayden", WithFilter(exclude /metrics))`.
- [ ] Manual: `/metrics` returns prometheus text incl. `hayden_*` after a `/force`. Commit: `feat: expose /metrics`

### Task 6: target-list UI on /

**Files:** `server/server.go`.

- [ ] Replace the static root template with an HTML table from `store.List`: name, url, last_status, last_run_at, last_match_at, last_error. **Do not render hook** (avoid leaking it on the public page). Escape via `html/template` (auto).
- [ ] Manual: `/` shows the table.
- [ ] Commit: `feat: list targets with run-state on /`

### Task 7: docs

**Files:** spec.

- [ ] Mark Phase 4/5 done; note jsonpath uses gjson path syntax and the css "selector exists" rule.
- [ ] `go build/vet/test` + full golangci-lint clean. Commit: `docs: phase 4/5 notes`

## Self-Review

Covers spec Phase 4 (css/regex/jsonpath, `change`, http fetch already shipped) and Phase 5 (`/metrics`, target-list `/`). Add-target HTML form deferred (API + token cover creation; a form needs token UX).

## Done when

`go test ./...` passes; `/metrics` exposes `hayden_*`; `/` lists targets with run-state; a target with `match_type` css/regex/jsonpath scans and matches; `notify_mode: change` re-notifies on content change.
