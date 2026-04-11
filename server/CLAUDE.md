# Go Server

HTTP server storing W3C Web Annotations in Postgres, serving them via REST API and MCP tools.

## Conventions

- Standard library `net/http` — no framework
- `internal/` for non-exported packages (handlers, storage, models)
- `cmd/server/` for the entrypoint

## Database

- Postgres via `pgx/v5` — no ORM
- Migrations: plain SQL files in `migrations/`, applied in order on server startup
- Connection string from `SERVER_DB_URL` env var

## W3C Web Annotation Storage

The `annotation` JSONB column is the canonical W3C envelope. Indexed columns (`project`, `domain`, `worktree`, `branch`, `state`, `motivation`) are denormalized for query performance only.

Never flatten W3C fields into top-level SQL columns. The JSONB column is the source of truth.

## Testing

Use scenarigo for HTTP integration tests against real Postgres. Do not mock the database.

## Running

```bash
just server       # go run ./cmd/server
just test-server  # go test ./...
just lint         # golangci-lint run
just fmt          # gofmt -w .
```
