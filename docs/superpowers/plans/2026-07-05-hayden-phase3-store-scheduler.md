# Hayden Phase 3 (Store + Scheduler + Webhook) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans. TDD throughout. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Make hayden functional end-to-end: targets live in Postgres (add/remove via API, no rebuild), a scheduler scans each on its own period, and a real webhook fires on a match — with run-state persisted.

**Architecture:** gorm + Postgres (reportd's `db.Connect`/`AutoMigrate` pattern). A `Fetcher` gets page content, a `Matcher` decides a match, a `Notifier` POSTs the webhook; `Scan` orchestrates them and persists run-state via a `Store`. A `Scheduler` runs one ticker per enabled target. The server wires it together, seeds from the embedded `config.json` on first run, and exposes a targets CRUD API.

**Tech Stack:** Go 1.26, gorm + `gorm.io/driver/postgres`, `namsral/flag`, chi, chromedp (headless mode).

## Global Constraints

- Postgres only. DB URL from `namsral/flag` `database_url` with env prefix `HAYDEN` → env var `HAYDEN_DATABASE_URL`. Required; fatal if empty.
- No CGO (build stays `CGO_ENABLED=0`); the Postgres driver (pgx) is pure-Go, so do NOT add `gorm.io/driver/sqlite` (it needs CGO).
- Server still listens `:8080`, answers `GET /healthz`, single container.
- Match types beyond `substring`, the `http`↔`headless` distinction's headless correctness, and `notify_mode: change` are **Phase 4** — this plan ships `substring` + `once` + a working `http` fetch (and keeps the existing chromedp path as the `headless` adapter).
- Store/db tests run against `TEST_DATABASE_URL` and `t.Skip` when it is unset; pure-logic tests (match, notify decision, scan with a fake fetcher) need no DB.
- Never force-push; commit as follow-ups on the `phase3-store-scheduler` branch.

---

## File Structure

- `target.go` — **rewrite**: `Target` becomes the gorm model (spec data model) + small helpers (`EffectiveHook`, `EffectivePeriod`).
- `db.go` — **new**: `Connect(ctx, url)` (postgres), `AutoMigrate(ctx, db)`.
- `store.go` — **new**: `Store` wrapping `*gorm.DB`; `Create/List/Get/Delete/SaveRunState`.
- `match.go` — **new**: `Matcher` interface + `substringMatcher`; `MatcherFor(matchType)`.
- `fetch.go` — **rewrite from scrape.go**: `Fetcher` interface + `httpFetcher` (full) + `headlessFetcher` (adapter over existing chromedp).
- `notify.go` — **new**: `Notifier` (webhook POST) + `ShouldNotify(target, matched)` decision (`once`).
- `scan.go` — **rewrite from config.go's ScrapeTargets**: `Scanner` holding Store/Fetcher/Notifier; `Scan(ctx, target)`, `ScanAll(ctx)`.
- `scheduler.go` — **new**: `Scheduler` (per-target tickers, `Start/Stop/Reload`).
- `config.go` — **trim**: keep `Config` (defaults) + `SeedConfig` loader mapping legacy `text`→`match_value`.
- `server/server.go` — **rewrite**: namsral flags, connect+migrate+seed, start scheduler, targets CRUD API, `/force`→`ScanAll`, graceful shutdown.
- `cmd/migrate/main.go` — **new**: connect + AutoMigrate.
- `.github/workflows/test.yml` — **modify**: add Postgres 16 service + `TEST_DATABASE_URL`.

## Interfaces (locked signatures — tasks compose against these)

```go
// target.go
type Target struct {
    ID        uint           `gorm:"primaryKey" json:"id"`
    CreatedAt time.Time      `json:"created_at"`
    UpdatedAt time.Time      `json:"updated_at"`
    DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

    Name       string `gorm:"not null" json:"name"`
    URL        string `gorm:"not null" json:"url"`
    FetchMode  string `gorm:"not null;default:http" json:"fetch_mode"`   // http | headless
    MatchType  string `gorm:"not null;default:substring" json:"match_type"` // substring (Phase 4: css|regex|jsonpath)
    MatchValue string `json:"match_value"`
    Invert     bool   `json:"invert"`
    NotifyMode string `gorm:"not null;default:once" json:"notify_mode"`  // once (Phase 4: change)
    Hook       string `json:"hook,omitempty"`
    Period     int    `json:"period,omitempty"` // seconds; 0 → default
    Enabled    bool   `gorm:"not null;default:true" json:"enabled"`

    LastRunAt       *time.Time `json:"last_run_at,omitempty"`
    LastStatus      string     `json:"last_status,omitempty"`   // ok | error
    LastMatchAt     *time.Time `json:"last_match_at,omitempty"`
    LastMatched     bool       `json:"last_matched"`
    LastError       string     `json:"last_error,omitempty"`
    LastContentHash string     `json:"-"`
}
func (t *Target) EffectiveHook(cfg *Config) string
func (t *Target) EffectivePeriod(cfg *Config) time.Duration

// db.go
func Connect(ctx context.Context, databaseURL string) (*gorm.DB, error)
func AutoMigrate(ctx context.Context, db *gorm.DB) error

// store.go
type Store struct { DB *gorm.DB }
func NewStore(db *gorm.DB) *Store
func (s *Store) Create(ctx context.Context, t *Target) error
func (s *Store) List(ctx context.Context) ([]*Target, error)
func (s *Store) ListEnabled(ctx context.Context) ([]*Target, error)
func (s *Store) Get(ctx context.Context, id uint) (*Target, error)
func (s *Store) Delete(ctx context.Context, id uint) error
func (s *Store) SaveRunState(ctx context.Context, t *Target) error // updates the run-state columns
func (s *Store) Count(ctx context.Context) (int64, error)

// match.go
type Matcher interface { Match(content []byte, value string) (bool, error) }
func MatcherFor(matchType string) (Matcher, error)

// fetch.go
type Fetcher interface { Fetch(ctx context.Context, target *Target) ([]byte, error) }
func FetcherFor(fetchMode string, cfg *Config) (Fetcher, error)

// notify.go
type Notifier interface { Notify(ctx context.Context, t *Target, ev Event) error }
type Event struct {
    Target    string    `json:"target"`
    URL       string    `json:"url"`
    Matched   bool      `json:"matched"`
    MatchedAt time.Time `json:"matched_at"`
    MatchType string    `json:"match_type"`
}
type HTTPNotifier struct { Client *http.Client; DefaultHook string }
func ShouldNotify(t *Target, matched bool) bool // once: matched && !t.LastMatched

// scan.go
type Scanner struct { Store *Store; Cfg *Config; Notifier Notifier; Now func() time.Time }
func (sc *Scanner) Scan(ctx context.Context, t *Target) error // fetch→match→notify→SaveRunState
func (sc *Scanner) ScanAll(ctx context.Context) error

// scheduler.go
type Scheduler struct { Scanner *Scanner; Store *Store }
func (s *Scheduler) Start(ctx context.Context) error
func (s *Scheduler) Stop()
func (s *Scheduler) Reload(ctx context.Context) error
```

---

### Task 1: Target gorm model + db.Connect/AutoMigrate + deps

**Files:** rewrite `target.go`; new `db.go`; modify `go.mod`.

- [ ] **Step 1: Add deps**

```bash
go get gorm.io/gorm gorm.io/driver/postgres github.com/namsral/flag
```

- [ ] **Step 2: Rewrite `target.go`** to the `Target` struct above; add:

```go
func (t *Target) EffectiveHook(cfg *Config) string {
    if t.Hook != "" { return t.Hook }
    return cfg.DefaultHook
}
func (t *Target) EffectivePeriod(cfg *Config) time.Duration {
    secs := t.Period
    if secs <= 0 { secs = cfg.DefaultPeriod }
    if secs <= 0 { secs = 300 }
    return time.Duration(secs) * time.Second
}
```

Remove the old `Scan`/`url.Parse` method (superseded by `Scanner`).

- [ ] **Step 3: Write `db.go`** — copy reportd's `Connect` (postgres branch only) + `AutoMigrate(&Target{})`. Use `logger.Default.LogMode(logger.Warn)` and `PingContext`.

- [ ] **Step 4: Write the DB test** `db_test.go` (skips without `TEST_DATABASE_URL`):

```go
func testDB(t *testing.T) *gorm.DB {
    url := os.Getenv("TEST_DATABASE_URL")
    if url == "" { t.Skip("TEST_DATABASE_URL not set") }
    db, err := Connect(context.Background(), url)
    if err != nil { t.Fatal(err) }
    if err := AutoMigrate(context.Background(), db); err != nil { t.Fatal(err) }
    t.Cleanup(func() { db.Exec("DROP TABLE IF EXISTS targets") })
    return db
}
func TestConnectAndMigrate(t *testing.T) { _ = testDB(t) }
```

- [ ] **Step 5:** `go build ./...` (expect the old server/config references to break — they get fixed in later tasks; if so, temporarily comment the old `Find`/`ScrapeTargets` callers or proceed task-by-task keeping the tree building by stubbing). Prefer: keep `config.go`/`scrape.go`/`server` compiling by not deleting their symbols until their rewrite task. Commit when `go build ./...` is green.

Commit: `feat: add Target gorm model and Postgres connection`

### Task 2: Store CRUD

**Files:** new `store.go`, `store_test.go`.

- [ ] Write `store_test.go` (uses `testDB`): create a target, List returns it, Get by id, ListEnabled filters `enabled=false`, SaveRunState persists `LastStatus`/`LastMatched`, Delete soft-deletes (List no longer returns it), Count.
- [ ] Run — expect FAIL (no Store).
- [ ] Implement `store.go` per signatures (gorm `WithContext`, `Find`, `First`, `Create`, `Save`/`Updates` for run-state, `Delete`).
- [ ] Run — PASS. Commit: `feat: add target store CRUD`

### Task 3: Matcher (substring)

**Files:** new `match.go`, `match_test.go`.

- [ ] Write `match_test.go`: substring present → true; absent → false; `MatcherFor("substring")` ok; `MatcherFor("css")` returns error (not-yet-supported, Phase 4).
- [ ] Run — FAIL.
- [ ] Implement: `substringMatcher.Match` = `bytes.Contains(content, []byte(value))`; `MatcherFor` switch with `default: return nil, fmt.Errorf("unsupported match type %q", matchType)`.
- [ ] Run — PASS. Commit: `feat: add substring matcher`

### Task 4: Fetcher (http + headless adapter)

**Files:** rewrite `scrape.go`→`fetch.go`, `fetch_test.go`.

- [ ] Write `fetch_test.go`: `httptest.Server` returning `"hello world"`; `httpFetcher.Fetch` on a Target with that URL returns those bytes. (headless path is smoke-tested manually, not unit-tested.)
- [ ] Run — FAIL.
- [ ] Implement `httpFetcher` (uses `http.NewRequestWithContext`, a client with a timeout, reads body, `bodyclose`). Implement `headlessFetcher` by moving the existing chromedp `Navigate`+`InnerHTML(body)` logic here, returning `[]byte(htmlContent)`. `FetcherFor("http")`/`("headless")`; default error.
- [ ] Run — PASS. Commit: `feat: add http and headless fetchers`

### Task 5: Notifier + notify decision

**Files:** new `notify.go`, `notify_test.go`.

- [ ] Write `notify_test.go`: `ShouldNotify` truth table for `once` (matched && !LastMatched → true; matched && LastMatched → false; !matched → false). `HTTPNotifier.Notify` POSTs JSON to a `httptest.Server` that captures the body; assert payload fields + that empty `t.Hook` falls back to `DefaultHook`.
- [ ] Run — FAIL.
- [ ] Implement `ShouldNotify` and `HTTPNotifier.Notify` (`http.NewRequestWithContext` POST, `application/json`, `json.NewEncoder`, check status < 300, close body).
- [ ] Run — PASS. Commit: `feat: add webhook notifier and notify-once decision`

### Task 6: Scan orchestration

**Files:** rewrite `config.go`'s scrape parts into `scan.go`; new `scan_test.go`.

- [ ] Write `scan_test.go` with a `fakeFetcher` (returns canned bytes) and `fakeNotifier` (records calls). Wire a `Scanner` with a real `Store` (testDB) + fakes. Cases: match fires notify once and sets `LastMatched=true`/`LastMatchAt`/`LastStatus=ok`; second scan with still-matching content does NOT notify again (once); fetch error sets `LastStatus=error`+`LastError` and does not notify; `invert` flips.
- [ ] Run — FAIL.
- [ ] Implement `Scan`: `FetcherFor`→Fetch; on error persist error state, return. `MatcherFor`→Match; apply `Invert`. `matched := ...`. If `ShouldNotify` → build `Event`, `Notifier.Notify`. Update run-state fields, `SaveRunState`. `ScanAll`: `ListEnabled` then `Scan` each, continue-on-error, aggregate with `errors.Join`.
- [ ] Run — PASS. Commit: `feat: add scan orchestration`

### Task 7: Scheduler

**Files:** new `scheduler.go`, `scheduler_test.go`.

- [ ] Write `scheduler_test.go`: seed one enabled target with a tiny period via a `Now`/period injection; start scheduler with a fake fetcher that increments a counter; assert ≥2 scans within a short window; `Stop` halts further scans. Use small real durations (e.g. period 1s) and `Eventually`-style polling with a timeout, no `time.Sleep` races beyond a bounded wait.
- [ ] Run — FAIL.
- [ ] Implement: `Start` loads enabled targets, launches a goroutine per target with `time.NewTicker(EffectivePeriod)` that calls `Scanner.Scan`; track cancel funcs; `Stop` cancels all and waits (`sync.WaitGroup`); `Reload` = Stop + Start (called by the API after create/delete).
- [ ] Run — PASS. Commit: `feat: add per-target scheduler`

### Task 8: Server wiring + targets API

**Files:** rewrite `server/server.go`; trim `config.go` (keep `Config` + `SeedConfig`).

- [ ] Implement `SeedConfig(ctx, store, cf)` in `config.go`: if `store.Count()==0`, insert `cf.Targets` mapping legacy fields (`text`→`MatchValue`, `MatchType="substring"`, `FetchMode="headless"` to preserve old behavior, `NotifyMode="once"`, `Enabled=true`).
- [ ] Rewrite `server/server.go`:
  - `namsral/flag` FlagSet prefix `HAYDEN`; `database_url` flag; fatal if empty. `PORT` via env (as today).
  - `Connect`→`AutoMigrate`→`NewStore`→parse embedded `config.json`→`SeedConfig`.
  - Build `Scanner` (Store, Config, `HTTPNotifier{Client: &http.Client{Timeout:...}, DefaultHook: cfg.DefaultHook}`) and `Scheduler`; `Start`; `Stop` on shutdown signal.
  - Routes: `/healthz`; `/` (keep simple HTML for now — target-list UI is Phase 5); `/metrics` deferred to Phase 5; `POST /force` → `go Scanner.ScanAll`; `GET /targets` (List, JSON); `POST /targets` (decode Target JSON, validate `URL`+`MatchType` via `MatcherFor`, Create, `Scheduler.Reload`, 201); `DELETE /targets/{id}` (Delete, Reload, 204).
  - Graceful shutdown: `signal.NotifyContext` + `srv.Shutdown` + `scheduler.Stop` (reportd pattern), keep `ReadHeaderTimeout`.
  - JSON via a small `writeJSON` helper (reportd's).
- [ ] Verify locally against a real Postgres (see "Local verification" below): build, run, `POST /targets`, `GET /targets`, `/force`, `/healthz`.
- [ ] Commit: `feat: wire store, scheduler, and targets API into the server`

### Task 9: cmd/migrate

**Files:** new `cmd/migrate/main.go`.

- [ ] Implement (reportd pattern, trimmed): namsral prefix `HAYDEN`, `database_url` flag, `Connect`+`AutoMigrate`, log done.
- [ ] `go build ./...`. Commit: `feat: add migrate command`

### Task 10: CI Postgres service + docs

**Files:** modify `.github/workflows/test.yml`; update the spec's icco.me note.

- [ ] Add a `postgres:16` service to `test.yml` (reportd pattern) and `TEST_DATABASE_URL: postgres://hayden:hayden@localhost:5432/hayden_test?sslmode=disable`.
- [ ] Update `docs/superpowers/specs/...-design.md` icco.me note: env is `HAYDEN_DATABASE_URL`; include the compose snippet (postgres service + env).
- [ ] `go build ./... && go vet ./...` and run the full golangci-lint set clean. Commit: `ci: run tests against postgres; docs: db env + compose note`

## Local verification (real Postgres)

```bash
docker run -d --rm --name hayden-pg -e POSTGRES_USER=hayden -e POSTGRES_PASSWORD=hayden -e POSTGRES_DB=hayden -p 5433:5432 postgres:16
export TEST_DATABASE_URL="postgres://hayden:hayden@localhost:5433/hayden?sslmode=disable"
go test ./...
# server smoke:
HAYDEN_DATABASE_URL="$TEST_DATABASE_URL" PORT=8123 go run ./server &
curl -s localhost:8123/healthz
curl -s -XPOST localhost:8123/targets -d '{"name":"t","url":"https://example.com","match_type":"substring","match_value":"Example","fetch_mode":"http"}'
curl -s localhost:8123/targets
```

## Self-Review

**Spec coverage (Phase 3 slice):** targets in Postgres (Task 1-2,8) ✓; add without rebuild via `POST /targets` (Task 8) ✓; scheduler per period (Task 7) ✓; webhook firing implemented (Task 5-6) ✓; substring match implemented (Task 3,6) ✓; seed from config.json (Task 8) ✓; `cmd/migrate` (Task 9) ✓; run-state persisted (Task 1,2,6) ✓. Deferred to Phase 4/5 (explicit): css/regex/jsonpath, `notify_mode: change`, headless-correctness, `/metrics`, target-list UI.

**Placeholder scan:** none — signatures fixed above; representative test code inline.

**Type consistency:** `Scanner`/`Store`/`Notifier`/`Fetcher`/`Matcher` names used consistently across tasks; `SaveRunState` and `ListEnabled` referenced identically in store, scan, scheduler.

## Done when

`go test ./...` passes (pure-logic locally; store/db against a Postgres); a local server backed by Postgres accepts `POST /targets`, lists them, scans on schedule, fires a webhook on a substring match, and persists run-state — no image rebuild to add a target.
