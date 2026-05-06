# Annotation Platform

Self-hosted annotation platform where developers capture visual and technical observations from the browser during development. Annotations are stored as W3C Web Annotations in Postgres, pushed in real-time into active Claude Code sessions via channels, and triaged by agents. See [ROADMAP.md](ROADMAP.md) for full scope.

## Running

```bash
just up        # Start Postgres (Docker Compose)
just server    # Run Go server (native, not Docker) — also serves /mcp + claude/channel notifications
just down      # Stop Postgres
just reset     # Drop volumes and restart Postgres
just logs      # Tail Docker Compose logs
just status    # Docker Compose process status
```

## Ports

| Service | Port | Env var |
|---------|------|---------|
| Go server (HTTP + MCP) | 8090 | SERVER_PORT |
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

## Design Reference

Side panel design handoff bundle from Claude Design lives at `/tmp/havi-design/havi-human-agent-visual-interface/`:

- `README.md` — handoff instructions (read first)
- `chats/chat1.md` — conversation transcript with the design assistant (intent / what the user actually wants)
- `project/HAVI Side Panel.html` — primary v1 design (open by default)
- `project/HAVI Side Panel v2.html` — iterated v2 design
- `project/sidepanel.css`, `project/sidepanel.v2.css`, `project/colors_and_type.css` — styles
- `project/components/*.jsx` — React-style component breakdowns (AppShell, Annotation, Capture, Filters, Icons; v2 variants alongside)
- `project/assets/` — `logo.svg`, `marco.jpeg`
- `project/fonts/` — JetBrains Mono + Space Grotesk webfonts
- `project/tweaks-panel.jsx` — design controls (reference only)

These are prototypes — recreate visually in the extension, don't copy prototype structure verbatim. `/tmp` is volatile; if the bundle is gone, re-fetch from `https://api.anthropic.com/v1/design/h/5TXQ9H2pRCbgkgQMulx78A`.
