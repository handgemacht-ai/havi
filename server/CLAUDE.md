# Go Server

HTTP server storing W3C Web Annotations in Postgres, serving them via REST API and MCP tools.

## Conventions

- Standard library `net/http` — no framework
- `internal/` for non-exported packages (handlers, storage, models)
- `cmd/server/` for the entrypoint

## Database

- Default: SQLite via `modernc.org/sqlite` (pure Go, no cgo) at `~/.havi/havi.db`
- Opt-in: Postgres via `pgx/v5` (set `HAVI_DB_URL=postgres://...`)
- Migrations: plain SQL files in `migrations/sqlite/` and `migrations/postgres/`, selected by URL scheme, applied in order on server startup
- Connection string from `HAVI_DB_URL` env var (`SERVER_DB_URL` accepted for backward compat)

## W3C Web Annotation Storage

The `annotation` column (Postgres JSONB / SQLite TEXT with `json_valid` check) is the canonical W3C envelope. Indexed columns (`project`, `domain`, `worktree`, `branch`, `state`, `motivation`) are denormalized for query performance only.

Never flatten W3C fields into top-level SQL columns. The JSON column is the source of truth.

## MCP Endpoint

Mounted at `/mcp` (HTTP Streamable transport via `go-sdk`). Three tools:

- `list_annotations` — list/filter annotations (wraps `AnnotationService.List`)
- `get_annotation_image` — get screenshot as base64 image (wraps `AnnotationService.GetImage`)
- `resolve_annotation` — mark annotation resolved with metadata (wraps `AnnotationService.Resolve`)

Claude Code discovers the server via `../.mcp.json`.

## Testing

Use scenarigo for HTTP integration tests against real storage. Tests default to a SQLite tempfile (no Docker required); set `HAVI_TEST_PG_URL=postgres://...` to also exercise the Postgres backend. Do not mock the database.

## Running

```bash
just server       # go run ./cmd/server
just test-server  # go test ./...
just lint         # golangci-lint run
just fmt          # gofmt -w .
```
