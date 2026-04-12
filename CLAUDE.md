# Annotation Platform

Self-hosted annotation platform where developers capture visual and technical observations from the browser during development. Annotations are stored as W3C Web Annotations in Postgres, pushed in real-time into active Claude Code sessions via channels, and triaged by agents. See [ROADMAP.md](ROADMAP.md) for full scope.

## Running

```bash
just up        # Start Postgres (Docker Compose)
just server    # Run Go server (native, not Docker)
just channel   # Run channel server (normally spawned by Claude Code via .mcp.json)
just down      # Stop Postgres
just reset     # Drop volumes and restart Postgres
just logs      # Tail Docker Compose logs
just status    # Docker Compose process status
```

## Ports

| Service | Port | Env var |
|---------|------|---------|
| Go server | 8090 | SERVER_PORT |
| Channel server | 8091 | CHANNEL_PORT |
| Postgres | 5432 | DB_PORT |

See `.worktree-ports.json` for worktree offset configuration.

## Testing

```bash
just test         # Run all tests
just test-server  # Go server tests only
just lint         # golangci-lint
just fmt          # gofmt
```

## API Contract

Read [API.md](API.md) before writing any endpoint or API call. API.md is the single source of truth for request/response shapes. (Created by Epic 1 — may not exist yet.)

## Rules

- DO NOT add comments for what can be inferred by git diffs
- Ensure services fail fast if configuration (environment variables) is missing
- Read API.md before writing any endpoint or API call
