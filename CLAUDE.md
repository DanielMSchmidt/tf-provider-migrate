# Refinery Context (tfprovidermigrate)

> **Recovery**: Run `gt prime` after compaction, clear, or new session

Full context is injected by `gt prime` at session start.

## Quick Reference

- Check MQ: `gt mq list`
- Process next: `gt mq process`

## Quality Gates (required after each change)

- `go test ./...`
- `go test -tags=integration ./internal/migrate -run TestRealProviders`
