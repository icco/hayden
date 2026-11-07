# AGENTS.md

Guidance for coding agents working on hayden.

## Project Overview

A Go service (`go.icco.me/hayden`) that stores watch targets in Postgres, fetches each on a schedule (plain HTTP or headless Chrome), and POSTs a webhook when the page matches. `server/` is the HTTP API, `cmd/migrate` syncs the schema.

## Commands

```sh
go build ./...
go vet ./...
TEST_DATABASE_URL=postgres://... go test ./...   # store-backed tests skip without it
docker build .                                   # production image, includes headless-shell
```

## Conventions

- Standard Go code conventions and idiomatic error handling.
- PR titles and commits must follow Conventional Commits with lowercase subjects.
- Ensure all tests pass before submitting PRs.
