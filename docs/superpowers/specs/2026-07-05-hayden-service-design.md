# Design: Promote `hayden` to a full-time service

Date: 2026-07-05
Status: Proposed

## Goal

Turn `hayden` from a stale, half-implemented scraper into a publishable, modern,
useful web-watch service that publishes `ghcr.io/icco/hayden:main` and serves
`https://hayden.timeclimbers.com`. Hosting (compose service, DNS, uptime
monitor, reverse proxy) is already wired up in `icco.me`; this repo just needs
to become production-grade.

## Current state (as-is)

- Go HTTP service (`server/`) on `:8080` with routes `/`, `/healthz`, `/force`.
- `hayden` library: `Config`/`ConfigFile`, `Target`, `Scan`, `Find`.
- **Core matching is a stub**: `scrape.go`'s `scanHTMLContent` returns
  `"not implemented"`.
- **Webhook firing is not implemented**: `ConfigFile.ScrapeTargets` only *logs*
  on match; it never POSTs the hook.
- Targets + default hook live in an **embedded** `server/static/config.json`
  (currently zero targets; default hook `https://relay.natwelch.com/hook`).
- Bundles `chromedp/headless-shell` via `start.sh` for JS-rendered pages.
- No database. No scheduler — scans only run on manual `GET /force`.
- Stale infra: `Dockerfile` pins `golang:1.17`; `go.mod` already drifted to
  `go 1.23.0`; only a dead `.travis.yml`, no GitHub Actions.

## Deploy contract (fixed by the icco.me PR — do not change)

- Image: `ghcr.io/icco/hayden:main`
- Listens on `:8080`, reverse-proxied by caddy at `hayden.timeclimbers.com`
- Single **app** container (headless-shell + server together, as today). A
  Postgres sidecar service in compose is a separate service and does not
  violate this.

## Template

`icco/reportd` is the reference for the modern icco service shape and is copied
throughout:

- Multi-arch `docker.yml` (push-by-digest → manifest merge → publishes `:main`;
  dry-run + attestation gating on branch).
- `test.yml` with a Postgres service, `go-version-file: go.mod`, coverage gate.
- `golangci-lint`, `codeql-analysis`, `pr-title`, `yaml-json` workflows.
- gorm data layer + `cmd/migrate`.
- prometheus/OTel `/metrics`.
- `namsral/flag` for env-backed flags; `gutil/logging` for zap logging.

## Key decisions

1. **Datastore: Postgres (required), via gorm `gorm.io/driver/postgres`.**
   `DATABASE_URL` is required; there is no SQLite fallback. pgx is pure-Go, so
   the server builds as a static, CGO-free binary that drops cleanly into the
   headless-shell image. Targets and run-state both live in Postgres.
2. **Final image stays `chromedp/headless-shell`, pinned to a digest** (not
   alpine), because the contract requires headless-shell + server in one
   container. The Go binary is built in a `golang:1.26` builder stage and copied
   in; `start.sh` still launches headless-shell + server.
3. **Targets are data, not code.** Managed through a small HTTP API (and an
   HTML form on `/`), stored in Postgres. Adding a watch requires no rebuild.
   The embedded/mounted `config.json` becomes a one-time **seed** on first run.
4. **A real scheduler** replaces reliance on manual `/force`: a per-target
   ticker driven by each target's `period` (fallback `default-period`).
5. **Implement matching and webhook delivery from scratch** (both are currently
   stubs), then broaden matching.

## Architecture

### Package layout

Keep today's split: `hayden` (library) + `server/` (main), mirroring reportd.

- **`hayden/` (library)**
  - `target.go` — `Target` gorm model (see data model below) with CRUD helpers.
  - `store.go` — gorm DB open (Postgres), `AutoMigrate`, target CRUD, run-state
    writes.
  - `fetch.go` (renamed from `scrape.go`) — `Fetcher` interface with two modes:
    - `headless` — chromedp against headless-shell (today's behavior; for
      JS-rendered pages). Returns rendered `body` HTML.
    - `http` — plain `net/http` GET (faster/cheaper; required for JSON/API
      targets). Returns raw body bytes.
  - `match.go` — `Matcher` interface + implementations:
    - `substring` — current `Text` contains (default; back-compatible).
    - `css` — goquery selector; matches if selector exists, or (if
      `match_value` has a `selector::text`-style value) if the selected text
      contains the expected string. Keep the rule simple and documented.
    - `regex` — Go `regexp` against fetched content.
    - `jsonpath` — JSON-path evaluation against fetched JSON body.
    - `invert` flips the boolean result.
  - `notify.go` — `Notifier`: POSTs a JSON payload to the target's `hook`
    (fallback `default-hook`). Owns the **notify decision**:
    - `once` — fire only on a no-match → match *transition*; reset when it
      returns to no-match.
    - `change` — fire whenever the matched content hash differs from
      `last_content_hash`.
  - `scan.go` — orchestrates one target scan: fetch → match (+invert) → notify
    decision → POST webhook → persist run-state. Replaces the `"not
    implemented"` stub and the log-only `ScrapeTargets`. A `ScanAll` runs every
    active target (used by `/force`), continuing past per-target errors (as the
    current code already does).
  - `scheduler.go` — `Scheduler` runs one goroutine/ticker per active target at
    its `period`; reconciles (add/remove/update tickers) when targets change via
    the API. Graceful shutdown on context cancel.
  - `config.go` — `Config` (`default-hook`, `default-period`) sourced from
    env/flags via `namsral/flag`, not embedded JSON. Seed loader reads an
    optional `config.json` (embedded default or mounted path) once at first run.
- **`server/` (main)** — wiring copied from reportd:
  - `gutil/logging` zap logger + middleware; `unrolled/secure` + `unrolled/render`.
  - prometheus/OTel meter provider + `/metrics`.
  - chi router. Routes:
    - `GET /healthz` — liveness (unchanged).
    - `GET /` — HTML: list targets with last-run time, last status, last match,
      last error; plus an add-target form.
    - `GET /metrics` — prometheus exposition.
    - `POST /force` — trigger `ScanAll` now (kept).
    - `GET /targets`, `POST /targets`, `DELETE /targets/{id}` — targets CRUD
      (JSON). This is how a watch is added without a rebuild.
  - Startup: connect Postgres → `AutoMigrate` → seed from `config.json` if the
    targets table is empty → start `Scheduler`.
- **`cmd/migrate/`** — standalone gorm `AutoMigrate` runner (reportd pattern),
  for manual/CI migrations. The server also auto-migrates on startup so a
  single-container deploy needs no separate init step.

### Data model

`targets` table (gorm model):

| field               | type       | notes                                            |
|---------------------|------------|--------------------------------------------------|
| `id`                | uint (PK)  | gorm default                                     |
| `created_at`/`updated_at` | time | gorm default                                     |
| `name`              | string     | human label for the UI                           |
| `url`               | string     | target URL                                       |
| `fetch_mode`        | string     | `headless` \| `http` (default `headless`)        |
| `match_type`        | string     | `substring` \| `css` \| `regex` \| `jsonpath`    |
| `match_value`       | string     | search string / selector / pattern / json path   |
| `invert`            | bool       | flip match result                                |
| `notify_mode`       | string     | `once` \| `change` (default `once`)              |
| `hook`              | string     | override webhook; empty → `default-hook`         |
| `period`            | int        | seconds; 0 → `default-period`                    |
| `enabled`           | bool       | scheduler skips disabled targets (default true)  |
| `last_run_at`       | *time      | run-state                                        |
| `last_status`       | string     | `ok` \| `error`                                  |
| `last_match_at`     | *time      | last time it matched                             |
| `last_matched`      | bool       | current match state (drives `once` transition)   |
| `last_error`        | string     | last error message                               |
| `last_content_hash` | string     | drives `change` notify mode                      |

`Target` keeps the existing JSON tags (`url`, `text`→`match_value`, `invert`,
`hook`, `period`) as much as possible so the seed `config.json` maps cleanly;
document the `text` → `match_value` + `match_type=substring` mapping in the
seed loader.

### Data flow

```
scheduler tick (per target.period)
  └─> scan.Scan(target)
        fetch (headless|http) ──> content
        match (+invert) ───────> matched bool, content hash
        notify decision:
          once   → fire if !last_matched && matched
          change → fire if matched && hash != last_content_hash
        POST webhook (target.hook || default-hook)
        persist run-state (last_run_at, last_status, last_match_at,
                           last_matched, last_error, last_content_hash)

POST /force ──> scan.ScanAll (same path, all enabled targets, now)
GET  /      ──> render targets + run-state from Postgres
```

### Webhook payload

POST JSON to the hook:

```json
{ "target": "<name>", "url": "<url>", "matched": true, "matched_at": "<rfc3339>", "match_type": "<type>" }
```

### Observability (task 5)

prometheus/OTel meter (reportd pattern), exposed at `/metrics`:

- `hayden_scans_total{target,status}` — counter.
- `hayden_matches_total{target}` — counter.
- `hayden_notifications_total{target,result}` — counter.
- `hayden_scan_duration_seconds{target}` — histogram.

Plus HTTP server metrics via `otelhttp`.

### Error handling

- Per-target fetch/match/notify errors are recorded in `last_error` +
  `last_status=error` and counted; they never abort sibling targets or the
  scheduler.
- Webhook delivery failures are logged + counted and retried on the next tick.
- Missing/invalid `DATABASE_URL` is fatal at startup (fail fast).

## Docker & CI

- **`Dockerfile`**: stage 1 `golang:1.26` builder — `CGO_ENABLED=0 go build
  -ldflags="-s -w" -o /server ./server` (+ `/migrate ./cmd/migrate`). Stage 2
  `chromedp/headless-shell@sha256:<pinned>` — copy `/server`, `/migrate`,
  `start.sh`; keep `ENTRYPOINT ["./start.sh"]`. The exact digest is resolved
  during implementation and pinned.
- **`.github/workflows/`** adapted from reportd:
  - `docker.yml` — multi-arch (amd64 `ubuntu-24.04`, arm64 `ubuntu-24.04-arm`),
    push-by-digest, merge → manifest → tags; publishes `:main` on push to main,
    dry-run on PRs, attestation on main.
  - `test.yml` — Postgres 16 service, `go-version-file: go.mod`, coverage gate.
  - `golangci-lint.yml`, `codeql-analysis.yml`, `pr-title.yml`, `yaml-json.yml`.
- **Delete** `.travis.yml`. Refresh README badges (drop Travis, add Actions).

## Testing

- `match_test.go` — table tests for substring/css/regex/jsonpath, including
  `invert`.
- `notify_test.go` — pure-function tests for the `once` vs `change` decision
  against synthetic run-state; no network.
- `scan_test.go` — scan orchestration with a fake `Fetcher` and a fake
  `Notifier`, asserting run-state transitions.
- `store_test.go` — target CRUD + migrations against the CI Postgres service
  (`TEST_DATABASE_URL`), reportd-style.
- headless fetching is exercised via the `Fetcher` interface being faked in unit
  tests; a thin real-chromedp path is smoke-tested manually in the `docker run`
  check.

## Phasing (5 PRs, mapping to the priority list)

1. **Ship it** — add `docker.yml` + `test.yml` + lint/codeql/pr-title/yaml-json;
   delete `.travis.yml`; refresh README. Minimal Dockerfile fix so the build
   passes and `:main` publishes. (No behavior change beyond building.)
2. **Modernize** — Dockerfile → `golang:1.26` builder + digest-pinned
   headless-shell; `go get -u ./...`; verify `docker build` + `docker run`
   locally answers `/healthz`.
3. **Configurable + scheduler** — gorm store, `cmd/migrate`, targets CRUD API,
   scheduler, `config.json` seed; **implement webhook firing + `substring`
   match** so the service is functional end-to-end.
4. **Broaden matching** — `css` / `regex` / `jsonpath`, `http` fetch mode,
   `notify_mode` (`once`/`change`).
5. **Observability + UI** — `/metrics`, target-list `/` with run-state + add
   form.

## icco.me follow-up (out of this repo, noted for Nat)

Add to the icco.me compose for hayden: a Postgres service + `DATABASE_URL` env
on the hayden container. Exact compose snippet to be provided when phase 3
lands. The fixed contract (image, `:8080`, single app container) is untouched.

## Done when

`docker build .` succeeds, CI publishes `:main`, the running container answers
`/healthz`, and a new watch target can be added (via `POST /targets`) without
rebuilding the image.
