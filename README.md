# HAVI

Self-hosted annotation platform. Capture visual + technical observations from any web page during development; route them to active Claude Code sessions for context-rich fixes.

Annotations are stored as [W3C Web Annotations](https://www.w3.org/TR/annotation-model/) (default: SQLite at `~/.havi/havi.db`; opt-in: Postgres) and pushed in real-time into your Claude Code session via MCP.

## Install

### Recommended: Claude Code plugin

```text
# in any Claude Code session
/plugin marketplace add handgemacht-ai/havi
/plugin install havi@havi
/havi-setup
```

Then restart Claude with the plugin loaded:

```bash
claude --dangerously-load-development-channels plugin:havi@havi
```

`/havi-setup` downloads the prebuilt server binary, starts it as a background daemon under `${CLAUDE_PLUGIN_DATA}`, and opens the [Chrome Web Store listing](https://chrome.google.com/webstore/detail/deedaihcndphkolmnfegjfjilcadncil) for the extension. Subsequent sessions auto-revive the daemon via the plugin's `SessionStart` hook.

If `/havi-setup` reports `operation not permitted` on bind, the Claude Code sandbox is blocking port 8090. Either run `havi serve --daemon` once from a terminal outside Claude (the SessionStart hook will keep it alive afterwards), or allow local binding in `.claude/settings.json`:

```json
{ "sandbox": { "network": { "allowLocalBinding": true },
               "excludedCommands": ["havi *"] } }
```

### Manual binary install

```bash
curl -fsSL https://raw.githubusercontent.com/handgemacht-ai/havi/main/scripts/install.sh | sh
havi serve --daemon
curl http://localhost:8090/health
```

The installer fetches the latest release from [GitHub releases](https://github.com/handgemacht-ai/havi/releases), verifies the checksum, and drops the binary into `~/.local/bin/havi` (override with `HAVI_INSTALL_DIR=...`). Builds available for darwin/linux × amd64/arm64.

### Browser extension

Install from the [Chrome Web Store](https://chrome.google.com/webstore/detail/deedaihcndphkolmnfegjfjilcadncil), then click the toolbar icon on any page to start capturing.

## MCP bridge (Codex / stdio-only clients)

Some MCP clients (such as OpenAI Codex CLI) communicate over stdio rather than HTTP. `havi mcp-bridge` is a thin subprocess that reads newline-delimited JSON-RPC frames from stdin, forwards them to the local havi server's `/mcp` endpoint using the Streamable HTTP MCP protocol, and writes each JSON-RPC response back to stdout. On the first request it automatically starts the havi daemon if it is not already running (opt out by setting `HAVI_NO_AUTO_REVIVE=1`). To wire it into Codex, add the following to `~/.codex/config.toml`:

```toml
[[mcp_servers]]
name = "havi"
command = "havi"
args  = ["mcp-bridge"]
```

## Storage

| Path | Purpose |
|------|---------|
| `${HAVI_DATA_DIR:-$HOME/.havi}/havi.db` | SQLite database (default) |
| `${HAVI_DATA_DIR:-$HOME/.havi}/havi.pid` | Daemon PID file |
| `${HAVI_DATA_DIR:-$HOME/.havi}/server.log` | Daemon log |

The Claude plugin sets `HAVI_DATA_DIR=${CLAUDE_PLUGIN_DATA}` so plugin-managed data stays inside the plugin's writable area.

To override the database location directly: `HAVI_DB_URL=/path/to/file.db` or `HAVI_DB_URL=sqlite:///path/to/file.db`.

## Postgres (opt-in)

```bash
just up   # start Postgres via Docker Compose
HAVI_DB_URL=postgres://annotations:dev@localhost:5432/annotations?sslmode=disable havi serve
```

`just down` stops it; `just reset` wipes the volume.

## Ports

| Service | Port | Env var |
|---------|------|---------|
| Go server (HTTP + MCP) | 8090 | `SERVER_PORT` |
| Postgres (opt-in) | 5432 | `DB_PORT` |

## Build from source

```bash
just server       # run the Go server with SQLite at ~/.havi/havi.db
just test         # run all tests
just lint         # golangci-lint
```

## Documentation

- [API.md](API.md) — REST + MCP contract (single source of truth for request/response shapes)
- [ROADMAP.md](ROADMAP.md) — full scope and milestones
- [CHANGELOG.md](CHANGELOG.md) — release notes
- [PRIVACY.md](PRIVACY.md) — data handling
