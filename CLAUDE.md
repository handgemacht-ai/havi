# Annotation Platform

Self-hosted annotation platform where developers capture visual and technical observations from the browser during development. Annotations are stored as W3C Web Annotations (default: SQLite at `~/.havi/havi.db`; opt-in: Postgres), pushed in real-time into active Claude Code sessions via channels, and triaged by agents. See [ROADMAP.md](ROADMAP.md) for full scope.

## Running

```bash
just server               # Run Go server with SQLite at ~/.havi/havi.db (default)
havi serve --daemon       # Run in background, PID at ~/.havi/havi.pid, log at ~/.havi/server.log
```

Postgres is opt-in:

```bash
just up                                                  # Start Postgres (Docker Compose)
HAVI_DB_URL=postgres://annotations:dev@localhost:5432/annotations?sslmode=disable just server
just down / just reset / just logs / just status         # Postgres lifecycle
```

## Storage

- Default: SQLite at `~/.havi/havi.db` (configurable via `HAVI_DB_URL=/path/to/file.db` or `sqlite:///path/to/file.db`)
- Opt-in: Postgres via `HAVI_DB_URL=postgres://...`
- Migrations live in `server/migrations/sqlite/` and `server/migrations/postgres/`; the active backend is selected by URL scheme.

## Ports

| Service | Port | Env var |
|---------|------|---------|
| Go server (HTTP + MCP) | 8090 | SERVER_PORT |
| Postgres (opt-in) | 5432 | DB_PORT |

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


<!-- BEGIN BEADS INTEGRATION v:1 profile:minimal hash:ca08a54f -->
## Beads Issue Tracker

This project uses **bd (beads)** for issue tracking. Run `bd prime` to see full workflow context and commands.

### Quick Reference

```bash
bd ready              # Find available work
bd show <id>          # View issue details
bd update <id> --claim  # Claim work
bd close <id>         # Complete work
```

### Rules

- Use `bd` for ALL task tracking — do NOT use TodoWrite, TaskCreate, or markdown TODO lists
- Run `bd prime` for detailed command reference and session close protocol
- Use `bd remember` for persistent knowledge — do NOT use MEMORY.md files

## Session Completion

**When ending a work session**, you MUST complete ALL steps below. Work is NOT complete until `git push` succeeds.

**MANDATORY WORKFLOW:**

1. **File issues for remaining work** - Create issues for anything that needs follow-up
2. **Run quality gates** (if code changed) - Tests, linters, builds
3. **Update issue status** - Close finished work, update in-progress items
4. **PUSH TO REMOTE** - This is MANDATORY:
   ```bash
   git pull --rebase
   bd dolt push
   git push
   git status  # MUST show "up to date with origin"
   ```
5. **Clean up** - Clear stashes, prune remote branches
6. **Verify** - All changes committed AND pushed
7. **Hand off** - Provide context for next session

**CRITICAL RULES:**
- Work is NOT complete until `git push` succeeds
- NEVER stop before pushing - that leaves work stranded locally
- NEVER say "ready to push when you are" - YOU must push
- If push fails, resolve and retry until it succeeds
<!-- END BEADS INTEGRATION -->
