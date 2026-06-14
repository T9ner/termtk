# TermTalk (termtk) Developer Guide

## Build and Run Commands
- Run client application: `go run cmd/termtalk/main.go`
- Run relay server: `go run cmd/termtalk-relay/main.go`
- Run tests: `go test ./...`
- Format code: `go fmt ./...`
- Lint code: `go vet ./...`
- Full project validation (fmt, lint, test, cross-compile): `go run scripts/validate.go`

## Agent skills

### Issue tracker

Issues and PRDs live as local markdown files under `.scratch/`. See `docs/agents/issue-tracker.md`.

### Triage labels

The standard label vocabulary is mapped in `docs/agents/triage-labels.md`.

### Domain docs

This is a single-context repository. See `docs/agents/domain.md`.
