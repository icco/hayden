# Hayden Foundation (Ship + Modernize) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `hayden` build with a modern toolchain and publish `ghcr.io/icco/hayden:main` via GitHub Actions, so the running container answers `/healthz`.

**Architecture:** Two-stage Docker build — a `golang:1.26` builder producing a static (CGO-free) server binary, copied into a digest-pinned `chromedp/headless-shell` final stage that runs headless-shell + server via `start.sh`. CI mirrors the icco pattern (gotak's single-arch `docker.yml`, reportd's `test.yml`/lint/codeql).

**Tech Stack:** Go 1.26, chromedp/headless-shell, GitHub Actions, ghcr.io.

## Global Constraints

- Image name: `ghcr.io/icco/hayden` — published tag on push to main is `main`.
- Service listens on `:8080`; must answer `GET /healthz` with `200`.
- Single app container: headless-shell + server together, entrypoint `start.sh`.
- Final image base MUST be `chromedp/headless-shell`, pinned to a `@sha256:` digest (not `:latest`, not alpine).
- Server binary builds with `CGO_ENABLED=0`.
- Never force-push; commit as follow-ups. Work on a branch, not `main`.

---

### Task 1: Update dependencies

**Files:**
- Modify: `go.mod`, `go.sum`

**Interfaces:**
- Produces: an up-to-date module graph that still compiles and vets clean.

- [ ] **Step 1: Create the working branch**

```bash
cd ~/Projects/hayden
git checkout -b foundation-ship-modernize
```

- [ ] **Step 2: Update all dependencies**

```bash
go get -u ./...
go mod tidy
```

- [ ] **Step 3: Verify it still builds, vets, and tests**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: no errors; test output shows `[no test files]` for the three packages.

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: update dependencies"
```

---

### Task 2: Modernize the Dockerfile

**Files:**
- Modify: `Dockerfile`
- Reference: `~/Projects/gotak/Dockerfile` (builder stage pattern), `start.sh` (unchanged)

**Interfaces:**
- Consumes: `./server` package (main), `start.sh`.
- Produces: an image that runs headless-shell + server, exposing `:8080`.

- [ ] **Step 1: Resolve the current headless-shell digest**

```bash
docker buildx imagetools inspect chromedp/headless-shell:latest --format '{{println .Manifest.Digest}}'
```
Record the printed `sha256:...` value; use it in Step 2 as `<DIGEST>`.
(If `imagetools` is unavailable: `docker pull chromedp/headless-shell:latest && docker inspect --format='{{index .RepoDigests 0}}' chromedp/headless-shell:latest`.)

- [ ] **Step 2: Rewrite the Dockerfile**

```dockerfile
# Build stage — static, CGO-free server binary.
FROM golang:1.26 AS builder

ENV GOPROXY="https://proxy.golang.org"
ENV CGO_ENABLED=0

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -ldflags="-s -w" -o /server ./server

# Final stage — headless-shell + server in one container (deploy contract).
FROM chromedp/headless-shell@sha256:<DIGEST>

LABEL org.opencontainers.image.source=https://github.com/icco/hayden
LABEL org.opencontainers.image.description="Watches web pages and fires a webhook on a match."
LABEL org.opencontainers.image.licenses=MIT

ENV NAT_ENV="production"
ENV PORT="8080"
EXPOSE 8080

WORKDIR /app
COPY --from=builder /server .
COPY start.sh .

ENTRYPOINT ["./start.sh"]
```

Note: `start.sh` invokes `./server`; keep it executable (`chmod +x start.sh` is already set in git).

- [ ] **Step 3: Build the image**

Run: `docker build -t hayden:test .`
Expected: build completes; final image `hayden:test` created.

- [ ] **Step 4: Run and probe /healthz**

```bash
docker run -d --rm -p 8080:8080 --name hayden-test hayden:test
sleep 3
curl -fsS http://localhost:8080/healthz; echo
docker stop hayden-test
```
Expected: `curl` prints `ok.` and exits `0`.

- [ ] **Step 5: Commit**

```bash
git add Dockerfile
git commit -m "build: modernize Dockerfile to go 1.26, pin headless-shell digest"
```

---

### Task 3: Add GitHub Actions, drop Travis, refresh README

**Files:**
- Create: `.github/workflows/docker.yml`, `.github/workflows/test.yml`, `.github/workflows/golangci-lint.yml`, `.github/workflows/codeql-analysis.yml`, `.github/workflows/pr-title.yml`
- Delete: `.travis.yml`
- Modify: `README.md`
- Reference: `~/Projects/gotak/.github/workflows/docker.yml`, `~/Projects/reportd/.github/workflows/{test,golangci-lint,codeql-analysis,pr-title}.yml`

**Interfaces:**
- Produces: on push to `main`, `ghcr.io/icco/hayden:main` is built and pushed.

- [ ] **Step 1: Add `docker.yml`** (single-arch, gotak pattern)

```yaml
name: Create and publish Docker image
on:
  push:
    branches:
      - main
  pull_request:
env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}
jobs:
  build-and-push-image:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
      attestations: write
      id-token: write
    steps:
      - name: Checkout repository
        uses: actions/checkout@v7
      - name: Log in to the Container registry
        uses: docker/login-action@v4
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      - name: Extract metadata (tags, labels) for Docker
        id: meta
        uses: docker/metadata-action@v6
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
      - name: Build and push Docker image
        id: push
        uses: docker/build-push-action@v7
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
        with:
          context: .
          push: ${{ github.ref == 'refs/heads/main' }}
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
      - name: Generate artifact attestation
        uses: actions/attest-build-provenance@v4
        with:
          subject-name: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          subject-digest: ${{ steps.push.outputs.digest }}
          push-to-registry: ${{ github.ref == 'refs/heads/main' }}
```

Note: `docker/metadata-action` tags a branch push with the branch name, so pushing `main` yields `ghcr.io/icco/hayden:main`.

- [ ] **Step 2: Add `test.yml`** (no Postgres yet — added in Phase 3)

```yaml
name: Test Go
on:
  - push
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
      - name: Setup Go
        uses: actions/setup-go@v6
        with:
          go-version-file: go.mod
      - name: Install dependencies
        run: go get ./...
      - name: Build
        run: go build -v ./...
      - name: Vet
        run: go vet ./...
      - name: Test
        run: go test -v ./...
```

- [ ] **Step 3: Copy lint/codeql/pr-title workflows from reportd**

```bash
cp ~/Projects/reportd/.github/workflows/golangci-lint.yml .github/workflows/golangci-lint.yml
cp ~/Projects/reportd/.github/workflows/codeql-analysis.yml .github/workflows/codeql-analysis.yml
cp ~/Projects/reportd/.github/workflows/pr-title.yml .github/workflows/pr-title.yml
```
Then open each and confirm there are no `reportd`-specific hardcoded paths/names (these three are generic; fix any if found).

- [ ] **Step 4: Run the linter locally and fix findings**

```bash
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest run ./... 2>&1 | head -40
```
Fix any reported issues (expected candidates: the `/` handler in `server/server.go:61-74` writes the template *and* an extra `"ok."`; `scrape.go:38` `scanHTMLContent` ignores `ctx`). Keep fixes minimal — only what the linter flags. Re-run until clean.

- [ ] **Step 5: Delete Travis and refresh README badges**

```bash
git rm .travis.yml
```
In `README.md`, remove the Travis build-status badge line and add:
```markdown
[![Create and publish Docker image](https://github.com/icco/hayden/actions/workflows/docker.yml/badge.svg)](https://github.com/icco/hayden/actions/workflows/docker.yml)
```

- [ ] **Step 6: Verify everything still builds/tests, then commit**

Run: `go build ./... && go vet ./... && go test ./...`
Expected: clean.

```bash
git add .github README.md
git add -u
git commit -m "ci: add GitHub Actions (docker publish, test, lint, codeql), drop Travis"
```

---

## Self-Review

**Spec coverage (Phase 1 + 2 of the spec):**
- "Add a GitHub Actions workflow that builds and pushes `:main`" → Task 3, Step 1. ✓
- "Delete `.travis.yml`" → Task 3, Step 5. ✓
- "Bump to current Go" → Task 2 (builder `golang:1.26`); `go.mod` already `go 1.23.0`, deps bumped in Task 1. ✓
- "`go get -u ./...`" → Task 1, Step 2. ✓
- "Pin the headless-shell base image to a digest" → Task 2, Steps 1–2. ✓
- "Confirm `docker build` + `docker run` works locally" → Task 2, Steps 3–4. ✓
- Lint/codeql/pr-title/test workflows (reportd parity) → Task 3, Steps 2–3. ✓

Deferred to later plans (correctly out of Foundation scope): gorm/Postgres store, `cmd/migrate`, scheduler, targets API, real matching, webhook firing, `/metrics`, target-list UI, `test.yml` Postgres service.

**Placeholder scan:** `<DIGEST>` in Task 2 is resolved by Task 2 Step 1's command before use — not a plan placeholder. No TODO/TBD elsewhere.

**Type consistency:** No Go type changes in this plan (infra only), beyond minimal linter fixes.

## Done when (this plan)

`docker build .` succeeds locally and answers `/healthz` on `docker run`; `.github/workflows/docker.yml` exists and (once merged to main) publishes `ghcr.io/icco/hayden:main`; `.travis.yml` is gone.
