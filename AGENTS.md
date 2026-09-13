# AGENTS.md

Guidance for coding agents working on hayden.

## Project Overview

A Go package and tool (`github.com/icco/hayden`) for image processing, color clustering, and palette extraction.

## Commands

```sh
go test ./...    # Run tests
go vet ./...     # Vet code
go build .       # Build package / tool
```

## Conventions

- Standard Go code conventions and idiomatic error handling.
- PR titles and commits must follow Conventional Commits with lowercase subjects.
- Ensure all tests pass before submitting PRs.
